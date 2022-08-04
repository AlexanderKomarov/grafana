package grpcserver

import (
	"context"
	"testing"

	apikeygenprefix "github.com/grafana/grafana/pkg/components/apikeygenprefixed"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/apikey"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestAuthenticator_Authenticate(t *testing.T) {
	serviceAccountId := int64(1)
	t.Run("accepts service api key with admin role", func(t *testing.T) {
		s := newFakeAPIKey(&apikey.APIKey{
			Id:               1,
			OrgId:            1,
			Key:              "admin-api-key",
			Name:             "Admin API Key",
			ServiceAccountId: &serviceAccountId,
		}, nil)
		a := NewAuthenticator(s, &fakeSQLStore{OrgRole: models.ROLE_ADMIN})
		ctx, err := setupContext()
		require.NoError(t, err)
		ctx, err = a.Authenticate(ctx)
		require.NoError(t, err)
	})

	t.Run("rejects non-admin role", func(t *testing.T) {
		s := newFakeAPIKey(&apikey.APIKey{
			Id:               1,
			OrgId:            1,
			Key:              "admin-api-key",
			Name:             "Admin API Key",
			ServiceAccountId: &serviceAccountId,
		}, nil)
		a := NewAuthenticator(s, &fakeSQLStore{OrgRole: models.ROLE_EDITOR})
		ctx, err := setupContext()
		require.NoError(t, err)
		ctx, err = a.Authenticate(ctx)
		require.NotNil(t, err)
	})
}

type fakeAPIKey struct {
	apikey.Service
	key *apikey.APIKey
	err error
}

func newFakeAPIKey(key *apikey.APIKey, err error) *fakeAPIKey {
	return &fakeAPIKey{
		key: key,
		err: err,
	}
}

func (f *fakeAPIKey) GetAPIKeyByHash(ctx context.Context, hash string) (*apikey.APIKey, error) {
	return f.key, f.err
}

type fakeSQLStore struct {
	sqlstore.Store
	OrgRole models.RoleType
}

func (f *fakeSQLStore) GetSignedInUserWithCacheCtx(ctx context.Context, query *models.GetSignedInUserQuery) error {
	query.Result = &models.SignedInUser{
		UserId:  1,
		OrgId:   1,
		OrgRole: f.OrgRole,
	}
	return nil
}

func setupContext() (context.Context, error) {
	ctx := context.Background()
	key, err := apikeygenprefix.New("sa")
	if err != nil {
		return ctx, err
	}
	md := metadata.New(map[string]string{})
	md["authorization"] = []string{"Bearer " + key.ClientSecret}
	return metadata.NewIncomingContext(ctx, md), nil
}
