package interceptors

import (
	"context"
	"strings"

	"github.com/grafana/grafana/pkg/cmd/grafana-cli/logger"
	"github.com/grafana/grafana/pkg/infra/log"
	xctx "github.com/grafana/grafana/pkg/infra/x/context"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/apikey"
	"github.com/grafana/grafana/pkg/services/auth/jwt"

	grpccontext "github.com/grafana/grafana/pkg/services/grpcserver/context"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/user"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Authenticator interface {
	Authenticate(ctx context.Context) (context.Context, error)
}

// authenticator can authenticate GRPC requests.
type authenticator struct {
	contextHandler grpccontext.ContextHandler
	logger         log.Logger

	UserService          user.Service
	AccessControlService accesscontrol.Service
	PluginAuthService    jwt.PluginAuthService
}

func ProvideAuthenticator(apiKeyService apikey.Service, userService user.Service, accessControlService accesscontrol.Service, contextHandler grpccontext.ContextHandler, pluginAuthService jwt.PluginAuthService) Authenticator {
	return &authenticator{
		contextHandler: contextHandler,
		logger:         log.New("grpc-server-authenticator"),

		AccessControlService: accessControlService,
		UserService:          userService,
	}
}

// Authenticate checks that a token exists and is valid, and then removes the token from the
// authorization header in the context.
func (a *authenticator) Authenticate(ctx context.Context) (context.Context, error) {
	return a.tokenAuth(ctx)
}

const tokenPrefix = "Bearer "

func (a *authenticator) tokenAuth(ctx context.Context) (context.Context, error) {
	auth, err := extractAuthorization(ctx)
	if err != nil {
		return ctx, err
	}

	if !strings.HasPrefix(auth, tokenPrefix) {
		return ctx, status.Error(codes.Unauthenticated, `missing "Bearer " prefix in "authorization" value`)
	}

	token := strings.TrimPrefix(auth, tokenPrefix)
	if token == "" {
		return ctx, status.Error(codes.Unauthenticated, "token required")
	}

	newCtx := purgeHeader(ctx, "authorization")

	signedInUser, err := a.getSignedInUser(ctx, token)
	if err != nil {
		logger.Warn("request with invalid token", "error", err, "token", token)
		return ctx, status.Error(codes.Unauthenticated, "invalid token")
	}

	newCtx = a.contextHandler.SetUser(newCtx, signedInUser)

	return newCtx, nil
}

func (a *authenticator) getSignedInUser(ctx context.Context, token string) (*user.SignedInUser, error) {
	claims, err := a.PluginAuthService.Verify(ctx, token)
	if err != nil {
		return nil, err
	}

	subject, ok := claims["subject"].(string)
	if !ok || subject == "" {
		return nil, status.Error(codes.Unauthenticated, "token missing subject claim")
	}

	userInfo := xctx.UserInfoFromString(subject)
	if userInfo != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid subject claim")
	}

	querySignedInUser := user.GetSignedInUserQuery{UserID: userInfo.UserID, OrgID: userInfo.OrgID}
	signedInUser, err := a.UserService.GetSignedInUserWithCacheCtx(ctx, &querySignedInUser)
	if err != nil {
		return nil, err
	}

	if signedInUser == nil {
		return nil, status.Error(codes.Unauthenticated, "service account not found")
	}

	if !signedInUser.HasRole(org.RoleAdmin) {
		return nil, status.Error(codes.PermissionDenied, "service account does not have admin role")
	}

	// disabled service accounts are not allowed to access the API
	if signedInUser.IsDisabled {
		return nil, status.Error(codes.PermissionDenied, "service account is disabled")
	}

	if signedInUser.Permissions == nil {
		signedInUser.Permissions = make(map[int64]map[string][]string)
	}

	if signedInUser.Permissions[signedInUser.OrgID] == nil {
		permissions, err := a.AccessControlService.GetUserPermissions(ctx, signedInUser, accesscontrol.Options{})
		if err != nil {
			a.logger.Error("failed fetching permissions for user", "userID", signedInUser.UserID, "error", err)
		}
		signedInUser.Permissions[signedInUser.OrgID] = accesscontrol.GroupScopesByAction(permissions)
	}

	return signedInUser, nil
}

func extractAuthorization(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "no headers in request")
	}

	authHeaders, ok := md["authorization"]
	if !ok {
		return "", status.Error(codes.Unauthenticated, `no "authorization" header in request`)
	}

	if len(authHeaders) != 1 {
		return "", status.Error(codes.Unauthenticated, `malformed "authorization" header: one value required`)
	}

	return authHeaders[0], nil
}

func purgeHeader(ctx context.Context, header string) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	mdCopy := md.Copy()
	mdCopy[header] = nil
	return metadata.NewIncomingContext(ctx, mdCopy)
}
