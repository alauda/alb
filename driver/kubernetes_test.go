package driver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"alauda.io/alb2/config"
)

func TestCreateDriver(t *testing.T) {
	a := assert.New(t)

	config.Set("TEST", "true")
	drv, err := GetDriver(context.Background())
	a.NoError(err)

	a.NotNil(drv.Client)
}
