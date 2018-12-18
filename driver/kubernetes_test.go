package driver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"alb2/config"
)

func TestCreateDriver(t *testing.T) {
	a := assert.New(t)

	config.Set("TEST", "true")
	drv, err := GetDriver()
	a.NoError(err)

	a.NotNil(drv.Client)
}
