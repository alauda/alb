package test_utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNgxConfig(t *testing.T) {
	cfg := `

events {
    multi_accept        on;
    worker_connections  51200;
}

http {
	server {
		listen 127.0.0.1:81;
		listen 192.168.2.3:81;
	}
}

stream {
	server {
		listen 127.0.0.1:80 udp;
		listen 192.168.2.3:80 udp;
	}
	server {
		listen 127.0.0.1:80;
		listen 192.168.2.3:80;
	}
}
`
	listen, err := PickStreamServerListen(cfg)
	assert.NoError(t, err)
	assert.Equal(t, listen[0], "127.0.0.1:80 udp")
	assert.Equal(t, listen[1], "192.168.2.3:80 udp")
	assert.Equal(t, listen[2], "127.0.0.1:80")
	assert.Equal(t, listen[3], "192.168.2.3:80")
	assert.Equal(t, len(listen), 4)

	listen, err = PickHttpServerListen(cfg)
	assert.NoError(t, err)
	assert.Equal(t, listen[0], "127.0.0.1:81")
	assert.Equal(t, listen[1], "192.168.2.3:81")
	assert.Equal(t, len(listen), 2)
}
