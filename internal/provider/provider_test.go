package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProviderModel_PortHandling(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		port         int64
		expectedHost string
	}{
		{
			name:         "standard port",
			host:         "localhost",
			port:         9030,
			expectedHost: "localhost:9030",
		},
		{
			name:         "custom port",
			host:         "starrocks.example.com",
			port:         8030,
			expectedHost: "starrocks.example.com:8030",
		},
		{
			name:         "different host and port",
			host:         "192.168.1.100",
			port:         9999,
			expectedHost: "192.168.1.100:9999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := fmt.Sprintf("%d", tt.port)
			hostPort := fmt.Sprintf("%s:%s", tt.host, port)
			if hostPort != tt.expectedHost {
				t.Errorf("hostPort = %v, want %v", hostPort, tt.expectedHost)
			}
		})
	}
}

func TestProviderModel_Fields(t *testing.T) {
	model := starrocksProviderModel{
		Host:     types.StringValue("localhost"),
		Port:     types.Int64Value(9030),
		Username: types.StringValue("root"),
		Password: types.StringValue("password"),
	}

	if model.Host.ValueString() != "localhost" {
		t.Errorf("Host = %v, want localhost", model.Host.ValueString())
	}
	if model.Port.ValueInt64() != 9030 {
		t.Errorf("Port = %v, want 9030", model.Port.ValueInt64())
	}
	if model.Username.ValueString() != "root" {
		t.Errorf("Username = %v, want root", model.Username.ValueString())
	}
	if model.Password.ValueString() != "password" {
		t.Errorf("Password = %v, want password", model.Password.ValueString())
	}
}

func TestProviderModel_HostPortCombination(t *testing.T) {
	model := starrocksProviderModel{
		Host:     types.StringValue("starrocks.local"),
		Port:     types.Int64Value(9030),
		Username: types.StringValue("admin"),
		Password: types.StringValue("secret"),
	}

	host := model.Host.ValueString()
	port := fmt.Sprintf("%d", model.Port.ValueInt64())
	hostPort := fmt.Sprintf("%s:%s", host, port)

	expected := "starrocks.local:9030"
	if hostPort != expected {
		t.Errorf("hostPort = %v, want %v", hostPort, expected)
	}
}
