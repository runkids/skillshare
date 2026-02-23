package install

import "testing"

func TestShouldFallbackTrackedClone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "partial clone unsupported",
			err:  errString("fatal: server does not support filter"),
			want: true,
		},
		{
			name: "dumb http shallow unsupported",
			err:  errString("fatal: dumb http transport does not support shallow capabilities"),
			want: true,
		},
		{
			name: "auth error does not fallback",
			err:  errString("fatal: Authentication failed for 'https://example.com/repo.git'"),
			want: false,
		},
		{
			name: "repo not found does not fallback",
			err:  errString("fatal: repository 'https://example.com/repo.git' not found"),
			want: false,
		},
		{
			name: "generic error does not fallback",
			err:  errString("fatal: could not resolve host"),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldFallbackTrackedClone(tt.err); got != tt.want {
				t.Fatalf("shouldFallbackTrackedClone() = %v, want %v", got, tt.want)
			}
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }
