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
  ContextKeys []ContextKey
  Fields []Field
}

type ContextKey struct {
  Value string
}

type Field struct {
  Key string
  Value string
}

var app = &App{}

func GetActions() []*Action {
  return app.Actions
}

func InitActions() {
  app.Actions = make([]*Action, 0)
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

  txt, _ := ioutil.ReadFile(filename)

  // Inspect the AST for public exported methods of the controller.
  ast.Inspect(f, func(n ast.Node) bool {
    switch x := n.(type) {
    case *ast.FuncDecl:
      if x.Recv != nil {
        if fmt.Sprintf("%v", x.Recv.List[0].Type) == ctrlName {
          a := &Action{
            Controller: ctrlName,
            Name: x.Name.Name,
          }

          a.Fields = make([]Field, 0)
          if len(x.Type.Params.List) > 0 {
            for _, obj := range x.Type.Params.List {
              a.Fields = append(a.Fields, Field{
                Key: fmt.Sprintf("%v", obj.Names[0]),
                Value: fmt.Sprintf("%v", obj.Type),
              })
            }
          }

          log.Printf("FIELD!: %v", a.Fields)

          rx, _ := regexp.Compile(fmt.Sprintf("%s[\\w\\W]*?http\\.Context{([a-zA-Z0-9, ]+)}", x.Name.Name))
          wrx, _ := regexp.Compile(fmt.Sprintf("%s[\\w\\W]*?http\\.Context{([a-zA-Z0-9, \\W]+?)}", x.Name.Name))
          ctxs := rx.FindAllStringSubmatch(string(txt), -1)
          if len(ctxs) > 0 {
            log.Printf("first worked!!")
            if len(ctxs[0]) > 1 {
              str := strings.Split(ctxs[0][1], ",")
              a.ContextKeys = make([]ContextKey, 0)
              for _, val := range str {
                a.ContextKeys = append(a.ContextKeys, ContextKey{
                  Value: strings.Trim(val, " "),
                })
              }
            }
          }

          if (a.ContextKeys == nil) {
            wctxs := wrx.FindAllStringSubmatch(string(txt), -1)
            if len(wctxs) > 0 {
              if len(wctxs[0]) > 1 {
                str := strings.Split(wctxs[0][1], ",\n")
                str = str[0:len(str) - 1]
                log.Printf("SECOND WORKED!!!! %v", str)
                a.ContextKeys = make([]ContextKey, 0)
                for _, val := range str {
                  a.ContextKeys = append(a.ContextKeys, ContextKey{
                    Value: strings.Trim(val, "\n\t ,"),
                  })
                }
              }
            }
          }

          log.Printf("CONTEXT KEYS: %v", a)
          app.Actions = append(app.Actions, a)
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