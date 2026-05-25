package diff_test

import (
	"fmt"

	"github.com/opendecree/decree/sdk/tools/diff"
)

func ExampleCompare() {
	old := map[string]string{
		"app.env":  "staging",
		"app.port": "8080",
	}
	cur := map[string]string{
		"app.env":   "production",
		"app.debug": "false",
	}

	result := diff.Compare(old, cur)
	fmt.Println(result.HasChanges())
	fmt.Println(len(result.ByType(diff.Added)))
	fmt.Println(len(result.ByType(diff.Modified)))
	fmt.Println(len(result.ByType(diff.Removed)))
	// Output:
	// true
	// 1
	// 1
	// 1
}

func ExampleResult_Format() {
	old := map[string]string{"app.env": "staging"}
	cur := map[string]string{"app.env": "production"}

	fmt.Print(diff.Compare(old, cur).Format())
	// Output:
	// ~ app.env: staging → production
}
