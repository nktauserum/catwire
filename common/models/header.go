//go:build amd64

package models

func pool_acquire(*uint64) uint32
func pool_release(*uint64, uint32)
