package deployer

import "math/big"

type Deployer interface {
	Deploy() error
	Set(valueToSet int64) error
	Get() (*big.Int, error)
}
