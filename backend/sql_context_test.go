package main

import (
 "context"
 "os"
 "os/exec"
 "path/filepath"
 "testing"
)

func TestDiscoverSqlTarget(t *testing.T) {
 if _,err:=exec.LookPath("git");err!=nil{t.Skip(err)}
 root:=t.TempDir()
 ctx:=context.Background()
 if _,err:=git(ctx,root,"init");err!=nil{t.Fatal(err)}
 dir:=filepath.Join(root,"migration")
 if err:=os.Mkdir(dir,0700);err!=nil{t.Fatal(err)}
 file:=filepath.Join(dir,"hello @ 中文.sql")
 if err:=os.WriteFile(file,[]byte("SELECT 1;\n"),0600);err!=nil{t.Fatal(err)}
 result,err:=handle(ctx,"vcs/discover",params{Kind:"file",Path:file})
 if err!=nil{t.Fatal(err)}
 got:=result.(sqlTarget)
 canonical,err:=filepath.EvalSymlinks(root)
 if err!=nil{t.Fatal(err)}
 if got.Repository.Kind!="git"||got.Repository.Path!=canonical||got.File!="migration/hello @ 中文.sql"{t.Fatalf("%+v",got)}
 folder,err:=discoverSqlTarget(ctx,params{Kind:"folder",Path:dir})
 if err!=nil||folder.Repository!=got.Repository||folder.File!=""{t.Fatalf("%+v %v",folder,err)}
 for _,p:=range []params{{Kind:"file",Path:"relative.sql"},{Kind:"folder",Path:file},{Kind:"file",Path:dir},{Kind:"invalid",Path:file},{Kind:"folder",Path:t.TempDir()}}{
  if _,err:=discoverSqlTarget(ctx,p);err==nil{t.Fatalf("accepted invalid context %+v",p)}
 }
 // Nested Git repositories must not be attributed to the outer repository.
 if _,err:=git(ctx,dir,"init");err!=nil{t.Fatal(err)}
 nested,err:=discoverSqlTarget(ctx,params{Kind:"file",Path:file})
 if err!=nil||nested.File!="hello @ 中文.sql"||nested.Repository.Path!=filepath.Join(canonical,"migration"){t.Fatalf("%+v %v",nested,err)}
}

func TestSQLDecorationBadges(t *testing.T){
 for _,tc:=range []struct{kind,status,badge string}{
  {"git"," M","M"},{"git","A ","A"},{"git","??","?"},{"git","UU","C"},{"git"," D","D"},{"git","R ","R"},{"git","!!",""},
  {"svn","modified","M"},{"svn","normal / props modified","M"},{"svn","modified / tree conflict","C"},{"svn","unversioned","?"},{"svn","missing","!"},{"svn","deleted","D"},
 }{
  if got:=sqlDecorationBadge(tc.kind,tc.status);got!=tc.badge{t.Errorf("%s %q => %q, want %q",tc.kind,tc.status,got,tc.badge)}
 }
}

func TestGitSQLDecorations(t *testing.T){
 if _,err:=exec.LookPath("git");err!=nil{t.Skip(err)}
 ctx:=context.Background()
 root:=t.TempDir()
 dir:=filepath.Join(root,"SQL")
 g:=func(args ...string){t.Helper();if _,err:=git(ctx,root,args...);err!=nil{t.Fatal(err)}}
 g("init");g("config","user.name","Test");g("config","user.email","test@example.invalid")
 if err:=os.Mkdir(dir,0700);err!=nil{t.Fatal(err)}
 write:=func(path,body string){t.Helper();if err:=os.WriteFile(path,[]byte(body),0600);err!=nil{t.Fatal(err)}}
 file:=filepath.Join(dir,"hello @ 中文.sql")
 write(file,"SELECT 1;\n");g("add",".");g("commit","-m","initial")
 write(file,"SELECT 2;\n")
 added:=filepath.Join(dir,"new.sql");write(added,"SELECT 3;\n");g("add","SQL/new.sql")
 write(filepath.Join(root,"outside.sql"),"SELECT 4;\n")
 v,err:=handle(ctx,"sqlFile/decorations/"+pluginID+".sql-folder",params{Kind:"folder",Path:dir})
 if err!=nil{t.Fatal(err)}
 got:=map[string]string{}
 for _,d:=range v.([]sqlDecoration){got[d.Path]=d.Badge}
 if len(got)!=2||got[file]!="M"||got[added]!="A"{t.Fatalf("%+v",got)}
 if _,err:=sqlDecorations(ctx,params{Kind:"file",Path:file});err==nil{t.Fatal("accepted file decoration scope")}
}
