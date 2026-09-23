/*
Copyright 2024 Telespazio UK.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPulsarConfigTopicName(t *testing.T) {
	if got := (PulsarConfig{}).TopicName(); got != DefaultPulsarTopic {
		t.Errorf("TopicName() with no topic = %q, want %q", got, DefaultPulsarTopic)
	}

	topic := "persistent://public/workspaces/workspace-controller"
	if got := (PulsarConfig{Topic: topic}).TopicName(); got != topic {
		t.Errorf("TopicName() = %q, want %q", got, topic)
	}
}

func TestConfigLoadPulsar(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want PulsarConfig
	}{
		{
			name: "url only",
			yaml: "pulsar:\n  url: pulsar://pulsar-proxy.pulsar:6650\n",
			want: PulsarConfig{URL: "pulsar://pulsar-proxy.pulsar:6650"},
		},
		{
			name: "token file and topic",
			yaml: "pulsar:\n" +
				"  url: pulsar://pulsar-proxy.pulsar:6650\n" +
				"  tokenFile: /var/run/secrets/pulsar/token\n" +
				"  topic: persistent://public/workspaces/workspace-controller\n",
			want: PulsarConfig{
				URL:       "pulsar://pulsar-proxy.pulsar:6650",
				TokenFile: "/var/run/secrets/pulsar/token",
				Topic:     "persistent://public/workspaces/workspace-controller",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			var c Config
			if err := c.Load(path); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if c.Pulsar != tt.want {
				t.Errorf("Load() pulsar = %+v, want %+v", c.Pulsar, tt.want)
			}
		})
	}
}

func TestNewEventsClientMissingTokenFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")

	_, err := NewEventsClient(PulsarConfig{
		URL:       "pulsar://127.0.0.1:6650",
		TokenFile: tokenFile,
	})
	if err == nil {
		t.Fatal("NewEventsClient() with a missing token file returned no error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("NewEventsClient() error = %v, want it to wrap fs.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), "tokenFile") || !strings.Contains(err.Error(), tokenFile) {
		t.Errorf("NewEventsClient() error = %q, want it to mention tokenFile and %q", err, tokenFile)
	}
}
