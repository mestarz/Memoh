package network

import "testing"

func TestDescriptorForBackendCapabilities(t *testing.T) {
	tests := []struct {
		name              string
		backend           string
		wantKind          string
		wantJoinContainer bool
	}{
		{
			name:              "docker",
			backend:           "docker",
			wantKind:          "docker",
			wantJoinContainer: true,
		},
		{
			name:              "empty defaults to docker",
			backend:           "",
			wantKind:          "docker",
			wantJoinContainer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := descriptorForBackend(tt.backend)
			if desc.Kind != tt.wantKind {
				t.Fatalf("Kind = %q, want %q", desc.Kind, tt.wantKind)
			}
			if desc.Capabilities.JoinContainerNetwork != tt.wantJoinContainer {
				t.Fatalf("JoinContainerNetwork = %v, want %v", desc.Capabilities.JoinContainerNetwork, tt.wantJoinContainer)
			}
		})
	}
}
