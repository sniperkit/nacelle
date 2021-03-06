package nacelle

import (
	"testing"

	"github.com/aphistic/sweet"
	"github.com/aphistic/sweet-junit"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	RegisterFailHandler(sweet.GomegaFail)

	sweet.Run(m, func(s *sweet.S) {
		s.RegisterPlugin(junit.NewPlugin())

		s.AddSuite(&ConfigSuite{})
		s.AddSuite(&ConfigTagsSuite{})
		s.AddSuite(&ServiceSuite{})
		s.AddSuite(&RunnerSuite{})
		s.AddSuite(&UtilSuite{})
	})
}
