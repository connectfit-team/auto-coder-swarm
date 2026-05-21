package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"
)

func main() {
	helpers := template.FuncMap{
		"lower": strings.ToLower,
	}
	
	// Create a minimal template that mimics task_detail.html
	const tpl = `{{define "content"}}<div class="tag-{{.Val | lower}}">{{.Val}}</div>{{end}}`
	
	t, err := template.New("test").Funcs(helpers).Parse(tpl)
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
