package core

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"r3w3/deployer"
	"r3w3/simplestorage"
)

type app struct {
	ctx      context.Context
	deployer deployer.Deployer
}

type AppConfigOpts struct {
	JsonRPCWebsocket string
	PrivateKeyString string
}

func NewApp(opts AppConfigOpts) (Core, error) {
	ctx := context.Background()
	d, err := deployer.NewDeployer(ctx, opts.JsonRPCWebsocket, opts.PrivateKeyString, deployer.WithLogEventHandler(logEvHandler))
	if err != nil {
		return nil, fmt.Errorf("could not create new deployer instance: %w", err)
	}

	return &app{
		ctx:      ctx,
		deployer: d,
	}, nil

}

func (a *app) SetNewValuesAtTimeInterval(valuesToSet []int64) error {
	if err := a.deployer.Deploy(); err != nil {
		return fmt.Errorf("could not deploy smart contract: %w", err)
	}

	for _, v := range valuesToSet {
		if err := a.deployer.Set(v); err != nil {
			return fmt.Errorf("could not set new value: %w", err)
		}

		_, err := a.deployer.Get()
		if err != nil {
			return fmt.Errorf("could not read new value: %w", err)
		}

		time.Sleep(time.Second * 3)
	}

	return nil
}

func logEvHandler(l types.Log) {
	fmt.Println("\n[NEW LOG]")
	contractAbi, err := abi.JSON(strings.NewReader(simplestorage.SimplestorageMetaData.ABI))
	if err != nil {
		log.Error("Could not get contract ABI: ", err.Error())
	}

	evDecoded, err := contractAbi.Unpack("Changed", l.Data)
	if err != nil {
		log.Error("could not unpack Changed event: ", err.Error())
	}

	for _, e := range evDecoded {
		switch e.(type) {
		case *big.Int:
			ev, _ := e.(*big.Int)
			fmt.Printf("VALUE CHANGED TO: %s\n\n", ev.String())
		}
	}
}
