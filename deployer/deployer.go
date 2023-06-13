package deployer

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"r3w3/simplestorage"
)

type deployer struct {
	ctx context.Context

	client *ethclient.Client

	privKey *ecdsa.PrivateKey
	pubKey  *ecdsa.PublicKey

	fromAddress common.Address
	txOpts      *bind.TransactOpts

	chainID *big.Int

	contractAddress       common.Address
	simpleStorageInstance *simplestorage.Simplestorage

	logEventHandler func(types.Log)
	logErrorHandler func(error)
}

func NewDeployer(context context.Context, jsonRPCWebsocket, privateKeyString string, opts ...func(*deployer)) (Deployer, error) {
	cl, err := ethclient.Dial(jsonRPCWebsocket)
	if err != nil {
		return nil, fmt.Errorf("could not dial json-rpc: %w", err)
	}

	privKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		return nil, fmt.Errorf("could not set private key: %w", err)
	}

	pubk, ok := privKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("could not set public key")
	}

	chainID, err := cl.ChainID(context)
	if err != nil {
		return nil, fmt.Errorf("could not get chain id: %w", err)
	}

	tOpts, err := bind.NewKeyedTransactorWithChainID(privKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("could not create transaction opts: %w", err)
	}

	d := &deployer{
		ctx:             context,
		client:          cl,
		privKey:         privKey,
		pubKey:          pubk,
		fromAddress:     crypto.PubkeyToAddress(*privKey.Public().(*ecdsa.PublicKey)),
		txOpts:          tOpts,
		chainID:         chainID,
		logErrorHandler: logsErrorHandler,
		logEventHandler: logsEventHandler,
	}

	for _, f := range opts {
		f(d)
	}

	return d, nil
}

func (d *deployer) Deploy() error {
	gas, err := d.client.SuggestGasPrice(d.ctx)
	if err != nil {
		return fmt.Errorf("could not estimate the gas price: %w", err)
	}

	d.txOpts.Nonce = d.getPendingNonce()
	d.txOpts.From = d.fromAddress
	d.txOpts.Value = big.NewInt(0)
	d.txOpts.GasLimit = uint64(5000000)
	d.txOpts.GasPrice = gas
	d.txOpts.Signer = d.signerHandler

	addr, tx, ss, err := simplestorage.DeploySimplestorage(d.txOpts, d.client)
	if err != nil {
		return fmt.Errorf("could not deploy smart contract: %w", err)
	}

	log.Printf("New smart contract deployed at: %s\n", addr.Hex())

	log.Println("Setting event listener")
	go d.setEventListener()

	d.contractAddress = addr
	d.simpleStorageInstance = ss

	_, err = d.waitForTxReceipt(tx.Hash())
	if err != nil {
		return err
	}

	log.Println("Simple Storage smart contract deployed successfully")

	return nil
}

func (d *deployer) Set(value int64) error {
	d.txOpts.Nonce = d.getPendingNonce()

	log.Println("Setting new value: ", value)

	setTx, err := d.simpleStorageInstance.Set(d.txOpts, big.NewInt(value))
	if err != nil {
		return fmt.Errorf("could not call Set function: %w", err)
	}

	_, err = d.waitForTxReceipt(setTx.Hash())
	if err != nil {
		return err
	}

	log.Printf("Smart contract value set to: %d\n", value)

	return nil
}

func (d *deployer) Get() (*big.Int, error) {
	d.txOpts.Nonce = d.getPendingNonce()

	getValue, err := d.simpleStorageInstance.Get(nil)
	if err != nil {
		return nil, fmt.Errorf("could not call Get smart contract function: %w", err)
	}

	return getValue, nil
}

func (d *deployer) signerHandler(_ common.Address, transaction *types.Transaction) (*types.Transaction, error) {
	signedTx, err := types.SignTx(transaction, types.NewEIP155Signer(d.chainID), d.privKey)
	if err != nil {
		return nil, fmt.Errorf("could not sign transaction: %w", err)
	}

	return signedTx, nil
}

func (d *deployer) getPendingNonce() *big.Int {
	nonce, err := d.client.PendingNonceAt(d.ctx, d.fromAddress)
	if err != nil {
		log.Println("Could not get pending nonce, transaction might fail: ", err.Error())
	}

	return big.NewInt(int64(nonce))
}

func (d *deployer) waitForTxReceipt(txHash common.Hash) (*types.Receipt, error) {
	tick := time.Tick(time.Second)
	timeOut := time.Tick(time.Minute)

	log.Printf("Waiting for transaction receipt: %s\n", txHash)

	for {
		select {
		case <-tick:
			receipt, _ := d.client.TransactionReceipt(d.ctx, txHash)
			if receipt != nil {
				return receipt, nil
			}
		case <-timeOut:
			return nil, fmt.Errorf("transaction receipt wait timeout")
		case <-d.ctx.Done():
			return nil, nil
		}
	}
}

func (d *deployer) setEventListener(queryOpts ...func(*ethereum.FilterQuery)) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{d.contractAddress},
	}

	for _, f := range queryOpts {
		f(&query)
	}

	logs := make(chan types.Log)

	sub, err := d.client.SubscribeFilterLogs(d.ctx, query, logs)
	if err != nil {
		d.logErrorHandler(err)
		return
	}

	for {
		select {
		case receivedLog := <-logs:
			d.logEventHandler(receivedLog)
		case subErr := <-sub.Err():
			d.logErrorHandler(subErr)
		case <-d.ctx.Done():
			return
		}
	}
}

func WithLogEventHandler(logFunc func(types.Log)) func(*deployer) {
	return func(d *deployer) {
		d.logEventHandler = logFunc
	}
}

func logsEventHandler(l types.Log) {
	fmt.Println("NEW LOG")
	fmt.Printf("%#v", l)
}

func logsErrorHandler(e error) {
	log.Printf("LOG EVENT ERROR: %s", e.Error())
}
