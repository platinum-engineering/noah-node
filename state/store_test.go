package state_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noah-blockchain/noah-node/types"
	cfg "github.com/tendermint/tendermint/config"
	dbm "github.com/tendermint/tm-db"
)

func TestStoreLoadValidators(t *testing.T) {
	stateDB := dbm.NewMemDB()
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})

	// 1) LoadValidators loads validators using a height where they were last changed
	SaveValidatorsInfo(stateDB, 1, 1, vals)
	SaveValidatorsInfo(stateDB, 2, 1, vals)
	loadedVals, err := LoadValidators(stateDB, 2)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())

	// 2) LoadValidators loads validators using a checkpoint height

	// TODO(melekes): REMOVE in 0.33 release
	// https://github.com/tendermint/tendermint/issues/3543
	// for releases prior to v0.31.4, it uses last height changed
	valInfo := &ValidatorsInfo{
		LastHeightChanged: ValSetCheckpointInterval,
	}
	stateDB.Set(CalcValidatorsKey(ValSetCheckpointInterval), valInfo.Bytes())
	assert.NotPanics(t, func() {
		SaveValidatorsInfo(stateDB, ValSetCheckpointInterval+1, 1, vals)
		loadedVals, err := LoadValidators(stateDB, ValSetCheckpointInterval+1)
		if err != nil {
			t.Fatal(err)
		}
		if loadedVals.Size() == 0 {
			t.Fatal("Expected validators to be non-empty")
		}
	})
	// ENDREMOVE

	SaveValidatorsInfo(stateDB, ValSetCheckpointInterval, 1, vals)

	loadedVals, err = LoadValidators(stateDB, ValSetCheckpointInterval)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())
}

func BenchmarkLoadValidators(b *testing.B) {
	const valSetSize = 100

	config := cfg.ResetTestRoot("state_")
	defer os.RemoveAll(config.RootDir)
	dbType := dbm.DBBackendType(config.DBBackend)
	stateDB := dbm.NewDB("state", dbType, config.DBDir())
	state, err := LoadStateFromDBOrGenesisFile(stateDB, config.GenesisFile())
	if err != nil {
		b.Fatal(err)
	}
	state.Validators = genValSet(valSetSize)
	state.NextValidators = state.Validators.CopyIncrementProposerPriority(1)
	SaveState(stateDB, state)

	for i := 10; i < 10000000000; i *= 10 { // 10, 100, 1000, ...
		SaveValidatorsInfo(stateDB, int64(i), state.LastHeightValidatorsChanged, state.NextValidators)

		b.Run(fmt.Sprintf("height=%d", i), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				_, err := LoadValidators(stateDB, int64(i))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
