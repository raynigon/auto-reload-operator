package controller

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getResource fetches the resource and writes it to the given reference
// if the resource exists, true is returned, false if not
func getResource(ctx context.Context, r client.Reader, name client.ObjectKey, obj client.Object, opts ...client.GetOption) (bool, error) {
	err := r.Get(ctx, name, obj)
	if err == nil {
		return true, nil
	}
	if errors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

func stringToNamespacedName(value string) (types.NamespacedName, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return types.NamespacedName{}, errors.New("invalid format")
	}
	return types.NamespacedName{
		Namespace: parts[0],
		Name:      parts[1],
	}, nil
}
