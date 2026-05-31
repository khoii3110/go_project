package auth

import "context"

type ctxKey string

const principalKey ctxKey = "principal"

type Principal struct {
	UserID string
	Role   string
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey).(Principal)
	return principal, ok
}
