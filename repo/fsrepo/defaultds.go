package fsrepo

import (
	"fmt"
	"path"

	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/thirdparty/dir"

	flatfs "gx/ipfs/QmQDvz6cULtMd1PuJgf7f2r75PqFuASXyGb7goXmJ4pPfr/go-ds-flatfs"
	ds "gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore"
	mount "gx/ipfs/QmSiN66ybp5udnQnvhb6euiWiiQWdGvwMhAWa95cC1DTCV/go-datastore/syncmount"
	ldbopts "gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	levelds "gx/ipfs/QmdXKJDw9Yk63NvtrA5a7g2zkeKQGKTWdDECLjNiSGSKT5/go-ds-leveldb"
	measure "gx/ipfs/QmeAJNAhWssP1Tt7ikNrjXMVxHJ96zhPvMxsCuNWJxmNe4/go-ds-measure"
)

const (
	leveldbDirectory = "datastore"
	flatfsDirectory  = "blocks"
)

func openDefaultDatastore(r *FSRepo) (repo.Datastore, error) {
	leveldbPath := path.Join(r.path, leveldbDirectory)

	// save leveldb reference so it can be neatly closed afterward
	leveldbDS, err := levelds.NewDatastore(leveldbPath, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to open leveldb datastore: %v", err)
	}

	syncfs := !r.config.Datastore.NoSync

	// 2 characters of base32 suffix gives us 10 bits of freedom.
	// Leaving us with 10 bits, or 1024 way sharding
	blocksDS, err := flatfs.CreateOrOpen(path.Join(r.path, flatfsDirectory), flatfs.NextToLast(2), syncfs)
	if err != nil {
		return nil, fmt.Errorf("unable to open flatfs datastore: %v", err)
	}

	prefix := "ipfs.fsrepo.datastore."
	metricsBlocks := measure.New(prefix+"blocks", blocksDS)
	metricsLevelDB := measure.New(prefix+"leveldb", leveldbDS)
	mountDS := mount.New([]mount.Mount{
		{
			Prefix:    ds.NewKey("/blocks"),
			Datastore: metricsBlocks,
		},
		{
			Prefix:    ds.NewKey("/"),
			Datastore: metricsLevelDB,
		},
	})

	return mountDS, nil
}

func initDefaultDatastore(repoPath string, conf *config.Config) error {
	// The actual datastore contents are initialized lazily when Opened.
	// During Init, we merely check that the directory is writeable.
	leveldbPath := path.Join(repoPath, leveldbDirectory)
	if err := dir.Writable(leveldbPath); err != nil {
		return fmt.Errorf("datastore: %s", err)
	}

	flatfsPath := path.Join(repoPath, flatfsDirectory)
	if err := dir.Writable(flatfsPath); err != nil {
		return fmt.Errorf("datastore: %s", err)
	}
	return nil
}
