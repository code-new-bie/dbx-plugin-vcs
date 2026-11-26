package main
import("context";"os";"os/exec";"path/filepath";"strings";"testing")
func TestGitChanges(t *testing.T){
 c,e:=gitChanges(" M a b.txt\x00R  new.txt\x00old.txt\x00?? 中文.txt\x00")
 if e!=nil||len(c)!=3||c[1].Original!="old.txt"||c[2].Path!="中文.txt"{t.Fatalf("%+v %v",c,e)}
}
func TestSVNChanges(t *testing.T){
 c,e:=svnChanges(`<status><target path="."><entry path="a.txt"><wc-status item="modified" props="modified"/></entry><entry path="b.txt"><wc-status item="conflicted" props="none" tree-conflicted="true"/></entry></target></status>`)
 if e!=nil||len(c)!=2||c[1].Status!="conflicted / tree conflict"{t.Fatalf("%+v %v",c,e)}
}
func TestPath(t *testing.T){
 root:=t.TempDir()
 for _,p:=range []string{"../outside","/etc/passwd",".","\\Windows\\System32","C:\\Windows\\System32","C:relative","\\\\server\\share\\file","//server/share/file","a/../../outside","a\\..\\..\\outside","a/../file","bad\x00file","bad\nfile"}{
  if _,e:=fileArg(root,p);e==nil{t.Errorf("accepted unsafe path %q",p)}
 }
 for _,p:=range []string{"","hello @ world.txt","中文.txt","nested/file.txt","-name.txt"}{
  got,e:=fileArg(root,p)
  want:=p
  if p!=""{want=filepath.Clean(p)}
  if e!=nil||got!=want{t.Errorf("path %q: got %q, error %v",p,got,e)}
 }
}
func TestSymlinkAncestor(t *testing.T){
 root:=t.TempDir()
 outside:=t.TempDir()
 if err:=os.Symlink(outside,filepath.Join(root,"linked"));err!=nil{t.Skipf("symlink unavailable: %v",err)}
 if _,err:=fileArg(root,"linked/file.txt");err==nil{t.Fatal("accepted symlinked ancestor")}
 if _,err:=fileArg(root,"linked");err!=nil{t.Fatalf("symlink entry itself must be permitted: %v",err)}
}

func TestRPC(t *testing.T){
 r:=respond([]byte(`{"jsonrpc":"2.0","id":1,"method":"plugin/initialize","params":{"host":{"protocolVersions":[1]}}}`))
 if r["error"]!=nil||r["result"]==nil{t.Fatal(r)}
 r=respond([]byte(`{"jsonrpc":"2.0","id":2,"method":"shell/exec"}`))
 if r["error"]==nil{t.Fatal(r)}
}
func TestGitIntegration(t *testing.T){
 if _,e:=exec.LookPath("git");e!=nil{t.Skip(e)}
 d:=t.TempDir();ctx:=context.Background()
 g:=func(a ...string){t.Helper();if _,e:=git(ctx,d,a...);e!=nil{t.Fatal(e)}}
 g("init");g("config","user.email","test@example.invalid");g("config","user.name","Test")
 f:=filepath.Join(d,"hello world.txt")
 if e:=os.WriteFile(f,[]byte("old\n"),0600);e!=nil{t.Fatal(e)}
 g("add",".");g("commit","-m","initial")
 if e:=os.WriteFile(f,[]byte("new\n"),0600);e!=nil{t.Fatal(e)}
 p:=params{Kind:"git",Path:d,File:"hello world.txt"}
 v,e:=handle(ctx,"vcs/status",p);if e!=nil||len(v.([]change))!=1{t.Fatalf("%v %v",v,e)}
 v,e=handle(ctx,"vcs/diff",p);if e!=nil||!strings.Contains(v.(map[string]string)["working"],"+new"){t.Fatalf("%v %v",v,e)}
 g("add",".")
 v,e=handle(ctx,"vcs/diff",p);if e!=nil||!strings.Contains(v.(map[string]string)["staged"],"+new"){t.Fatalf("%v %v",v,e)}
 v,e=handle(ctx,"vcs/log",p);if e!=nil||!strings.Contains(v.(string),"initial"){t.Fatalf("%v %v",v,e)}
 _,e=handle(ctx,"vcs/branches",p);if e!=nil{t.Fatal(e)}
}
