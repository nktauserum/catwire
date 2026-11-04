//go:build amd64

package models

import "time"

const initialBackoffValue = time.Nanosecond
const maxBackoffValue 	  = time.Millisecond

func pool_acquire(*uint64) uint32
func pool_release(*uint64, uint32)
