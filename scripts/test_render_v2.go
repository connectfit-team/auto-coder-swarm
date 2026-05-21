package main

import (
	"fmt"
	"html/template"
	"os"
)

func main() {
	// Minimal template WITHOUT custom functions
	const tpl = `{{define "content"}}<div>{{.Val}}</div>{{end}}`
	
	t, err := template.New("test").Parse(tpl)
	if err != nil {
		fmt.Printf("Parse Error: %v\n", err)
		return
	}
	
	data := map[string]string{"Val": "INIT"}
	err = t.ExecuteTemplate(os.Stdout, "content", data)
	if err != nil {
		fmt.Printf("Execute Error: %v\n", err)
	}
}
