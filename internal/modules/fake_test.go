package modules

import (
	wferrors "solace-semp-workflow/internal/errors"
)

// fakeClient is an in-memory Client for module tests. Each behavior can be
// programmed with an on<Verb> func; unset funcs use a benign default. Calls and
// payloads are recorded for assertions. It implements the same Client interface.
type fakeClient struct {
	onExists func(path string) (bool, map[string]any, error)
	onCreate func(path string, payload map[string]any) (map[string]any, error)
	onUpdate func(path string, payload map[string]any) (map[string]any, error)
	onDelete func(path string) error

	existsCalls  []string
	createPath   string
	createBody   map[string]any
	updatePath   string
	updateBody   map[string]any
	deletePath   string
	createdCount int
	updatedCount int
	deletedCount int
}

func (f *fakeClient) Exists(path string) (bool, map[string]any, error) {
	f.existsCalls = append(f.existsCalls, path)
	if f.onExists != nil {
		return f.onExists(path)
	}
	return false, nil, nil
}

func (f *fakeClient) Create(path string, payload map[string]any) (map[string]any, error) {
	f.createPath, f.createBody = path, payload
	f.createdCount++
	if f.onCreate != nil {
		return f.onCreate(path, payload)
	}
	return map[string]any{}, nil
}

func (f *fakeClient) Update(path string, payload map[string]any) (map[string]any, error) {
	f.updatePath, f.updateBody = path, payload
	f.updatedCount++
	if f.onUpdate != nil {
		return f.onUpdate(path, payload)
	}
	return map[string]any{}, nil
}

func (f *fakeClient) Delete(path string) error {
	f.deletePath = path
	f.deletedCount++
	if f.onDelete != nil {
		return f.onDelete(path)
	}
	return nil
}

// DefaultMsgVPN returns the VPN used by tests that do not name one explicitly.
func (f *fakeClient) DefaultMsgVPN() string { return "test-vpn" }

// existing returns a fake whose Exists always reports the resource is present.
func existing() *fakeClient {
	return &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return true, map[string]any{}, nil
	}}
}

// absent returns a fake whose Exists always reports the resource is absent.
func absent() *fakeClient {
	return &fakeClient{onExists: func(string) (bool, map[string]any, error) {
		return false, nil, nil
	}}
}

// sempErr builds a *SEMPError carrying the given SEMP code.
func sempErr(code int) error {
	return wferrors.NewSEMPError("api error", 400, code)
}
