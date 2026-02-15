package common

import (
	"fmt"

	"github.com/jr-k/d4s/internal/ui/styles"
)

func GetLogo() []string {
	p := styles.TagLogo        // primary
	s := styles.TagLogoShadow // shadow
	return []string{
		fmt.Sprintf(" [%s]██████[%s]╗[%s]   ██[%s]╗[%s]  ██[%s]╗[%s]   █████[%s]╗[%s] ", p, s, p, s, p, s, p, s, p),
		fmt.Sprintf(" [%s]██[%s]╔══[%s]██[%s]╗  [%s]██[%s]║  [%s]██[%s]║  [%s]██[%s]╔═══╝ ", p, s, p, s, p, s, p, s, p, s),
		fmt.Sprintf(" [%s]██[%s]║  [%s]██[%s]║  [%s]███████[%s]║  [%s]█████[%s]╗ ", p, s, p, s, p, s, p, s),
		fmt.Sprintf(" [%s]██[%s]║  [%s]██[%s]║       [%s]██[%s]║       [%s]██[%s]╗", p, s, p, s, p, s, p, s),
		fmt.Sprintf(" [%s]██████[%s]╔╝       [%s]██[%s]║  [%s]██████[%s]╔╝", p, s, p, s, p, s),
		fmt.Sprintf(" [%s]╚═════╝        ╚═╝  ╚═════╝ ", s),
	}
}
