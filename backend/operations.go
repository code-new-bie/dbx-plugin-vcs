package main

import (
 "context"
 "errors"
 "fmt"
 "strings"
)

// Only explicit actions are accepted; no shell or user-supplied options.
// Confirmed is a UI acknowledgement, not an authorization boundary.
func writeOperation(ctx context.Context, p params) (string, error) {
 if !p.Confirmed { return "", errors.New("explicit confirmation required") }
 r, err := inspect(ctx, p)
 if err != nil { return "", err }
 file, err := fileArg(r.Path, p.File)
 if err != nil { return "", err }
 message := strings.TrimSpace(p.Message)
 if p.Action == "commit" && (message == "" || len(message) > 16384 || strings.ContainsRune(message, 0)) { return "", errors.New("commit message required (max 16 KiB)") }
 switch r.Kind {
 case "git":
  switch p.Action {
  case "add", "unstage":
   if file == "" { return "", errors.New("select a changed file") }
   if p.Action == "add" { return git(ctx,r.Path,"add","--",file) }
   return git(ctx,r.Path,"restore","--staged","--",file)
  case "commit":
   if file != "" { return "", errors.New("Git commit applies to the entire staged index") }
   return git(ctx,r.Path,"commit","-m",message)
  case "fetch", "pull", "push":
   remote, ref, err := upstream(ctx,r.Path)
   if err != nil { return "",err }
   switch p.Action {
   case "fetch": return git(ctx,r.Path,"fetch","--no-recurse-submodules","--",remote)
   case "pull": return git(ctx,r.Path,"pull","--ff-only","--no-rebase","--no-autostash","--no-recurse-submodules","--",remote,ref)
   case "push": return git(ctx,r.Path,"push","--porcelain","--no-force","--no-follow-tags","--recurse-submodules=no","--",remote,"HEAD:"+ref)
   }
  }
 case "svn":
  switch p.Action {
  case "add", "commit":
   if file == "" { return "",errors.New("select a changed path") }
   if p.Action == "add" { return svn(ctx,r.Path,"add","--depth","empty","--",file+"@") }
   return svn(ctx,r.Path,"commit","--depth","empty","-m",message,"--",file+"@")
  case "update": return svn(ctx,r.Path,"update","--ignore-externals","--accept","postpone","--",".")
  }
 }
 return "",fmt.Errorf("unsupported action %q for %s",p.Action,r.Kind)
}

func upstream(ctx context.Context, path string) (string,string,error) {
 branch,err := git(ctx,path,"symbolic-ref","--quiet","--short","HEAD")
 if err != nil { return "","",errors.New("a checked-out branch with a configured upstream is required") }
 branch = strings.TrimSpace(branch)
 remote,err := git(ctx,path,"config","--get","branch."+branch+".remote")
 if err != nil { return "","",errors.New("configure the branch upstream in Git first") }
 ref,err := git(ctx,path,"config","--get","branch."+branch+".merge")
 remote,ref = strings.TrimSpace(remote),strings.TrimSpace(ref)
 if err != nil || remote == "" || remote == "." || strings.HasPrefix(remote,"-") || strings.ContainsAny(remote,"\r\n") || !strings.HasPrefix(ref,"refs/heads/") || strings.ContainsAny(ref,"\r\n") { return "","",errors.New("a single remote branch upstream is required") }
 if _,err = git(ctx,path,"remote","get-url","--",remote); err != nil { return "","",errors.New("configured remote not found") }
 if _,err = git(ctx,path,"check-ref-format",ref);err != nil { return "","",err }
 return remote,ref,nil
}
