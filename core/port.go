package core

type Core interface {
	SetNewValuesAtTimeInterval(valuesToSet []int64) error
}
