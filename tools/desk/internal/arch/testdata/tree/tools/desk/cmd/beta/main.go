// Command beta breaks R-dep-direction: one command importing another.
package main

import _ "example.com/fixture/tools/desk/cmd/alpha"

func main() {}
