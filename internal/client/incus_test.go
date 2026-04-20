package client

import "testing"

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "https endpoint", input: "https://127.0.0.1:8443", want: "https://127.0.0.1:8443"},
		{name: "http endpoint", input: "http://localhost:8443", want: "http://localhost:8443"},
		{name: "empty", input: "", wantErr: true},
		{name: "remote name not supported", input: "local", wantErr: true},
		{name: "bad url", input: "https://:8443", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeEndpoint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeEndpoint() err=%v wantErr=%v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizeEndpoint()=%q want=%q", got, tt.want)
			}
		})
	}
}
