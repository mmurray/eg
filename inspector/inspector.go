// Package inspector provides utilities to inspect an ego app directory, parse
// it's go files, and model it using ego metadata objects.
package inspector

import (
  "fmt"
  "strings"
  "io/ioutil"
  "path"
  "log"
  "regexp"
  "go/ast"
  "go/parser"
  "go/token"
)

type App struct {
  Actions []*Action
}

type Action struct {
  Controller string
  Name string
}

var app = &App{
  Actions: make([]*Action, 0),
}

func GetActions() []*Action {
  return app.Actions
}

func Inspect() {
  dirname := "app/controllers"
  dirlist, err := ioutil.ReadDir(dirname)
  if err != nil {
    log.Fatalf("Error reading %s: %s\n", dirname, err)
  }
  for _, f := range dirlist {
    filename := path.Join(dirname, f.Name())
    inspectFile(filename)
  }
}

func inspectFile(filename string) {
  ctrlName := getCtrlName(filename)
  if ctrlName == "" {
    return
  }
  
  fset := token.NewFileSet() // positions are relative to fset

  // Parse the file
  f, err := parser.ParseFile(fset, filename, nil, 0)
  if err != nil {
    panic(err)
  }

  // Inspect the AST for public exported methods of the controller.
  ast.Inspect(f, func(n ast.Node) bool {
    switch x := n.(type) {
    case *ast.FuncDecl:
      if x.Recv != nil {
        if fmt.Sprintf("%v", x.Recv.List[0].Type) == ctrlName {
          app.Actions = append(app.Actions, &Action{
            Controller: ctrlName,
            Name: x.Name.Name,
          })
        }
      }
    }
    return true
  })
}

func getCtrlName(filename string) string {
  pieces := strings.Split(filename, "/")
  file := pieces[len(pieces)-1]
  file = strings.Replace(file, ".go", "", 1)
  rxp, _ := regexp.Compile("_[a-zA-Z]*")
  str := rxp.ReplaceAllFunc([]byte(file), func(str []byte) []byte {
    name := string(str[1:])
    name = strings.ToUpper(name[0:1]) + name[1:]
    return []byte(name)
  })
  return strings.ToUpper(string(str[0:1])) + string(str[1:])
}