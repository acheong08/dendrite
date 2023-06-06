package tables_test

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"testing"

	"github.com/matrix-org/dendrite/internal/sqlutil"
	"github.com/matrix-org/dendrite/roomserver/storage/postgres"
	"github.com/matrix-org/dendrite/roomserver/storage/sqlite3"
	"github.com/matrix-org/dendrite/roomserver/storage/tables"
	"github.com/matrix-org/dendrite/roomserver/types"
	"github.com/matrix-org/dendrite/setup/config"
	"github.com/matrix-org/dendrite/test"
	"github.com/stretchr/testify/assert"
)

func mustCreateUserRoomKeysTable(t *testing.T, dbType test.DBType) (tab tables.UserRoomKeys, db *sql.DB, close func()) {
	t.Helper()
	connStr, close := test.PrepareDBConnectionString(t, dbType)
	db, err := sqlutil.Open(&config.DatabaseOptions{
		ConnectionString: config.DataSource(connStr),
	}, sqlutil.NewExclusiveWriter())
	assert.NoError(t, err)
	switch dbType {
	case test.DBTypePostgres:
		err = postgres.CreateUserRoomKeysTable(db)
		assert.NoError(t, err)
		tab, err = postgres.PrepareUserRoomKeysTable(db)
	case test.DBTypeSQLite:
		err = sqlite3.CreateUserRoomKeysTable(db)
		assert.NoError(t, err)
		tab, err = sqlite3.PrepareUserRoomKeysTable(db)
	}
	assert.NoError(t, err)

	return tab, db, close
}

func TestUserRoomKeysTable(t *testing.T) {
	test.WithAllDatabases(t, func(t *testing.T, dbType test.DBType) {
		tab, db, close := mustCreateUserRoomKeysTable(t, dbType)
		defer close()
		userNID := types.EventStateKeyNID(1)
		roomNID := types.RoomNID(1)
		_, key, err := ed25519.GenerateKey(nil)
		assert.NoError(t, err)

		err = sqlutil.WithTransaction(db, func(txn *sql.Tx) error {
			var gotKey, key2, key3 ed25519.PrivateKey
			gotKey, err = tab.InsertUserRoomKey(context.Background(), txn, userNID, roomNID, key)
			assert.NoError(t, err)
			assert.Equal(t, gotKey, key)

			// again, this shouldn't result in an error, but return the existing key
			_, key2, err = ed25519.GenerateKey(nil)
			assert.NoError(t, err)
			gotKey, err = tab.InsertUserRoomKey(context.Background(), txn, userNID, roomNID, key2)
			assert.NoError(t, err)
			assert.Equal(t, gotKey, key)

			// add another user
			_, key3, err = ed25519.GenerateKey(nil)
			assert.NoError(t, err)
			userNID2 := types.EventStateKeyNID(2)
			_, err = tab.InsertUserRoomKey(context.Background(), txn, userNID2, roomNID, key3)
			assert.NoError(t, err)

			gotKey, err = tab.SelectUserRoomKey(context.Background(), txn, userNID, roomNID)
			assert.NoError(t, err)
			assert.Equal(t, key, gotKey)

			// Key doesn't exist
			gotKey, err = tab.SelectUserRoomKey(context.Background(), txn, userNID, 2)
			assert.NoError(t, err)
			assert.Nil(t, gotKey)

			// query user NIDs for senderKeys
			var gotKeys map[string]types.EventStateKeyNID
			gotKeys, err = tab.BulkSelectUserNIDs(context.Background(), txn, [][]byte{key.Public().(ed25519.PublicKey), key3.Public().(ed25519.PublicKey)})
			assert.NoError(t, err)
			assert.NotNil(t, gotKeys)

			wantKeys := map[string]types.EventStateKeyNID{
				string(key.Public().(ed25519.PublicKey)):  userNID,
				string(key3.Public().(ed25519.PublicKey)): userNID2,
			}
			assert.Equal(t, wantKeys, gotKeys)
			return nil
		})
		assert.NoError(t, err)

	})
}
