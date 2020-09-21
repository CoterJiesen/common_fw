package client

// hash值计算器
type HashCalculator interface {
	Hash([]byte) uint64
}
