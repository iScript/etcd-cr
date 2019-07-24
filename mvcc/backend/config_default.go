// +build !linux,!windows

package backend

import bolt "go.etcd.io/bbolt"

var boltOpenOptions *bolt.Options

func (bcfg *BackendConfig) mmapSize() int { return int(bcfg.MmapSize) }
