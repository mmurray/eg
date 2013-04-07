// Package proxy provides an interface to a reverse-proxy that wraps the ego dev
// server. The reverse-proxy has functionality to make development easier (like
// automatically picking up file changes and displaying errors in the browser).
package proxy

import (
  "log"
  "net/http"
  "net/http/httputil"
  "net/url"
  "net"
  "os"
  "os/exec"
  "path"
  "strings"
  "fmt"
  "io"
  "strconv"
  "io/ioutil"
  "github.com/howeyc/fsnotify"
  "github.com/hoisie/mustache"
  "eg/templates"
  "eg/inspector"
  "bytes"
  "regexp"
)

type Proxy struct {
  dir string
  binPath string
  cmd []*exec.Cmd
  ln net.Listener
  conn net.Conn
}

func NewProxy() *Proxy {
  p := &Proxy{}
  p.initialize();
  return p;
}

var defaultProxy = NewProxy()

func Run() {
  defaultProxy.Run()
}

func (p *Proxy) initialize() {
  p.cmd = make([]*exec.Cmd, 0)
}

var errRegexp, _ = regexp.Compile("(.+go):([0-9]+):[0-9]{0,}:? (.+)")

func checkErr(err error) {
  if err != nil {
    log.Printf("ERR?")
    log.Printf("%s", err)
    log.Printf("...........")
  }
}

func watchAll(watcher *fsnotify.Watcher, dirname string) {
  dirlist, err := ioutil.ReadDir(dirname)
  if err != nil {
    log.Fatalf("Error reading %s: %s\n", dirname, err)
  }
  for _, f := range dirlist {
    filename := path.Join(dirname, f.Name())
    if f.IsDir() {
      log.Printf("watching: %v", filename)
      err = watcher.Watch(filename)
      checkErr(err)
      watchAll(watcher, filename)
    }
  }
}

func (p *Proxy) handleErr(err string) {
  log.Printf("ERRRRRR: %v", err)
    
    if errRegexp.MatchString(err) {
      pieces := errRegexp.FindStringSubmatch(err)
      log.Printf("%v", pieces)
      p.startErr(&ErrorHandler{
        Filename: pieces[1],
        Message: pieces[3],
        Line: pieces[2],
      })
    } else {
      fmt.Println("starting err..")
      p.startErr(&ErrorHandler{
        Filename: "",
        Message: err,
        Line: "",
      })
    }
}

func (p *Proxy) compile() bool {
  
  defer func() {
      if r := recover(); r != nil {
        fmt.Println("recover COMPILE")
        fmt.Println("recover %v", r)
          p.handleErr(fmt.Sprintf("%v", r))
      }
  }()

  log.Printf("compiling to: %v", p.binPath)
  log.Printf("go build -o %s %s", p.binPath, p.dir + "/server.go")
  cmd := exec.Command("go", "build", "-o", p.binPath, p.dir + "/server.go")
  log.Printf("AFTER EXEC COMMAND???")
  var buf bytes.Buffer
  stdout, err := cmd.StdoutPipe()
  checkErr(err)
  go io.Copy(&buf, stdout)
  // go io.Copy(os.Stderr, stderr)
  
  err = cmd.Start()
  checkErr(err)

  // log.Printf("removing... %s", p.dir)
  // os.RemoveAll(p.dir)
  log.Printf("waitin..")
  err = cmd.Wait()
  if err != nil {
    lines := strings.Split(buf.String(), "\n")
    if len(lines) >= 2 {
      p.handleErr(fmt.Sprintf("%v", lines[1]))
      return false
    }
  }
  log.Printf("compilllle err: %v", err)
  return true
  // if err != nil {
  //   p.startErr(&ErrorHandler{
  //     Filename: "",
  //     Message: "BLAH!",
  //     Line: "",
  //   })
  // }
    
}

func (p *Proxy) setupErrDir(e *ErrorHandler) {

  wd, _ := os.Getwd()
//  dirs := strings.Split(wd, "/")
 // curDir := dirs[len(dirs) - 1]

  root := ".ego-genfiles"
  os.Remove(root)
  os.MkdirAll(root, 0777)

  file, _ := ioutil.ReadFile(e.Filename)
  lines := strings.Split(string(file), "\n")

  line, _ := strconv.Atoi(e.Line)
  code := ""
  min := 0
  if (line - 6) > 0 {
    min = line - 6;
    code = fmt.Sprintf("<ol start='%v'>", line - 5);
  } else {
    code = "<ol>"
  }
  max := line + 5
  if (line + 5) > len(lines) {
    max = len(lines)
  }
  for i := min; i < max; i++ {
    num := i+1 
    value := lines[i]
    if num == line {
      code += "<li class='err'>"+value+"</li>"
    } else {
      code += "<li>"+value+"</li>"
    }
  }
  code += "</ol>"

  server := mustache.Render(string(templates.ErrServer()), map[string]interface{} {
    "Message": e.Message,
    "Filename": e.Filename,
    "Line": e.Line,
    "Code": code,
    "Port": ":5000",
  })
  serverFile, _ := os.Create(root+"/server.go")
  log.Printf("writing: %v", server)
  serverFile.Write([]byte(server))

  p.dir = path.Join(wd, root);
  p.binPath = path.Join(wd, root, "ego-server")
}

func (p *Proxy) setupDir() {

  wd, _ := os.Getwd()
  dirs := strings.Split(wd, "/")
  curDir := dirs[len(dirs) - 1]

  root := ".ego-genfiles"
  os.Remove(root)
  os.MkdirAll(root, 0777)

  server := mustache.Render(string(templates.Server()), map[string]interface{} {
    "Name": curDir,
    "Actions": inspector.GetActions(), 
  })
  serverFile, _ := os.Create(root+"/server.go")
  log.Printf("writing: %v", server)
  serverFile.Write([]byte(server))
  

  p.dir = path.Join(wd, root);
  p.binPath = path.Join(wd, root, "ego-server")
}

type ErrorHandler struct {
  Message string
  Filename string
  Line string
}

func (p *Proxy) startErr(e *ErrorHandler) {
  p.stop()
  fmt.Println("starting err!")
  p.setupErrDir(e)
  ok := p.compile()
  if !ok {
    fmt.Printf("NOT OK")
    return
  }
  log.Printf("running...")
  cmd := exec.Command(p.binPath, "-dev=true", fmt.Sprintf("-port=%v", /*p.flags["port"]*/ 5000))
  fmt.Printf("STARTING ERR: %v", cmd)
  p.cmd = append(p.cmd, cmd)
  fmt.Printf("APPENDING.. %v", len(p.cmd))
  stdout, err := cmd.StdoutPipe()
  checkErr(err)
  stderr, err := cmd.StderrPipe()
  checkErr(err)
  go io.Copy(os.Stdout, stdout)
  go io.Copy(os.Stderr, stderr)
  fmt.Print("start err")
  err = cmd.Start()
  checkErr(err)
  // log.Printf("removing... %s", p.dir)
  // os.RemoveAll(p.dir)
  err = cmd.Wait()

}

func (p *Proxy) start() {
  p.stop()
  fmt.Println("starting!!...")
  defer func() {
      if r := recover(); r != nil {
        fmt.Println("recover START")
        p.handleErr(fmt.Sprintf("%v", r))
      }
  }()
  inspector.InitActions()
  inspector.Inspect()
  p.setupDir()
  ok := p.compile()
  if !ok {
    fmt.Printf("NOT OK")
    return
  }
  log.Printf("running...")
  cmd := exec.Command(p.binPath, "-dev=true", fmt.Sprintf("-port=%v", /*p.flags["port"]*/ 5000))
  p.cmd = append(p.cmd, cmd)
  fmt.Printf("APPENDING... %v (from start)", len(p.cmd))
  stdout, err := cmd.StdoutPipe()
  checkErr(err)
  stderr, err := cmd.StderrPipe()
  checkErr(err)
  go io.Copy(os.Stdout, stdout)
  go io.Copy(os.Stderr, stderr)
  fmt.Print("start dev")
  err = cmd.Start()
  if (err != nil) {
    log.Printf("err: %v", err)
  }
  checkErr(err)
  // log.Printf("removing... %s", p.dir)
  // os.RemoveAll(p.dir)
  err = cmd.Wait()
}

func (p *Proxy) stop() {
  fmt.Println("killin..")
  log.Printf("%v", len(p.cmd))
  if len(p.cmd) > 0 {
    for _, cmd := range p.cmd {
      cmd.Process.Kill()
    }
    p.cmd = make([]*exec.Cmd, 0)
  }
    fmt.Println("after killin dev")
}

func (p *Proxy) run() {

  go p.start()

  watcher, err := fsnotify.NewWatcher()
  checkErr(err)
  err = watcher.Watch(".")
  checkErr(err)
  watchAll(watcher, "app")
  watchAll(watcher, "conf")
  // watchAll(watcher, ".")

  for {
      select {
      case evt := <-watcher.Event:
        if evt.Name != ".egoserver" {
          log.Print("EVENT!!!")
            p.stop()
            fmt.Println("restartin")
            go p.start()
        }
      case err := <-watcher.Error:
          log.Println("error:", err)
      }
  }
}
 
func (p *Proxy) Run() {

  go p.run()

  u, err := url.Parse("http://localhost:5000")
  if err != nil {
    log.Fatal(err)
  }
 
  reverse_proxy := httputil.NewSingleHostReverseProxy(u)
  http.Handle("/", reverse_proxy)
 
  log.Println("Server started")
  if err = http.ListenAndServe(":8080", nil); err != nil {
    log.Fatal(err)
  }
}

