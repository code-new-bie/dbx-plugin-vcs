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
