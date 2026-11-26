package main

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "strings"
)

type sqlTarget struct {
 Repository repo `json:"repository"`
 File string `json:"file,omitempty"`
}

// Read-only discovery uses the selected SQL file's parent or selected folder,
// not the DBX tree's root, so nested working copies resolve independently.
func discoverSqlTarget(ctx context.Context, p params) (sqlTarget,error) {
 if p.Kind!="file" && p.Kind!="folder" { return sqlTarget{},errors.New("expected file or folder context") }
 if !filepath.IsAbs(p.Path) { return sqlTarget{},errors.New("absolute SQL path required") }
 target:=filepath.Clean(p.Path)
 info,err:=os.Lstat(target)
 if err!=nil{return sqlTarget{},err}
 if info.Mode()&os.ModeSymlink!=0{return sqlTarget{},errors.New("symbolic-link SQL targets are not supported")}
 dir:=target
 if p.Kind=="file" {
  if !info.Mode().IsRegular(){return sqlTarget{},errors.New("regular SQL file required")}
  dir=filepath.Dir(target)
 } else if !info.IsDir(){return sqlTarget{},errors.New("directory required")}
 dir,err=filepath.EvalSymlinks(dir)
 if err!=nil{return sqlTarget{},err}
 if p.Kind=="file"{target=filepath.Join(dir,filepath.Base(target))}
 candidates:=[]repo{}
 for _,kind:=range []string{"git","svn"} {
  if ctx.Err()!=nil{return sqlTarget{},ctx.Err()}
  r,e:=inspect(ctx,params{Kind:kind,Path:dir})
  if e!=nil{continue}
  canonical,e:=filepath.EvalSymlinks(r.Path)
  if e!=nil{continue}
  r.Path=canonical
  rel,e:=filepath.Rel(r.Path,dir)
  if e!=nil||!filepath.IsLocal(rel){continue}
  candidates=append(candidates,r)
 }
 if len(candidates)==0{return sqlTarget{},errors.New("no Git or SVN working copy detected; check installed CLI tools and repository")}
 chosen:=candidates[0]
 if len(candidates)==2{
  a,b:=candidates[0],candidates[1]
  rel,e:=filepath.Rel(a.Path,b.Path)
  if e==nil&&rel=="."{return sqlTarget{},errors.New("both Git and SVN detected at the same root; select the repository manually")}
  // Prefer the innermost working copy when one repository contains another.
  if e==nil&&filepath.IsLocal(rel){chosen=b}
 }
 result:=sqlTarget{Repository:chosen}
 if p.Kind=="file" {
  rel,e:=filepath.Rel(chosen.Path,target)
  if e!=nil{return sqlTarget{},e}
  f,e:=fileArg(chosen.Path,rel)
  if e!=nil{return sqlTarget{},e}
  if strings.EqualFold(f,"."){return sqlTarget{},errors.New("invalid SQL file path")}
  result.File=filepath.ToSlash(f)
 }
 return result,nil
}

const decorationLimit = 10000
type sqlDecoration struct {
 Path string `json:"path"`
 Badge string `json:"badge"`
}

func sqlDecorationBadge(kind,status string) string {
 if kind=="git" {
  switch status {
  case "DD","AU","UD","UA","DU","AA","UU":return "C"
  case "??":return "?"
  case "!!":return ""
  }
  for _,code:=range []string{"U","D","R","A","C","M","T"}{
   if strings.Contains(status,code){
    switch code{case "U":return "C";case "C":return "A";case "T":return "M";default:return code}
   }
  }
  return ""
 }
 if strings.Contains(status,"conflict"){return "C"}
 switch strings.SplitN(status," / ",2)[0]{
 case "unversioned":return "?"
 case "added":return "A"
 case "deleted":return "D"
 case "replaced":return "R"
 case "missing","obstructed","incomplete":return "!"
 case "modified":return "M"
 }
 if strings.Contains(status,"props modified"){return "M"}
 return ""
}

// Only return descendants of the explicitly activated SQL folder. No SQL text,
// repository credentials or unrelated repository paths are returned to the host.
func sqlDecorations(ctx context.Context,p params)([]sqlDecoration,error){
 if p.Kind!="folder"{return nil,errors.New("SQL decoration scope must be a folder")}
 selected,err:=discoverSqlTarget(ctx,p)
 if err!=nil{return nil,err}
 scope,err:=filepath.EvalSymlinks(filepath.Clean(p.Path))
 if err!=nil{return nil,err}
 r:=selected.Repository
 value,err:=handle(ctx,"vcs/status",params{Kind:r.Kind,Path:r.Path})
 if err!=nil{return nil,err}
 rows,ok:=value.([]change)
 if !ok{return nil,errors.New("invalid status result")}
 result:=[]sqlDecoration{}
 for _,row:=range rows{
  badge:=sqlDecorationBadge(r.Kind,row.Status)
  if badge==""{continue}
  path:=filepath.FromSlash(row.Path)
  if !filepath.IsLocal(path){continue}
  rel,e:=filepath.Rel(scope,filepath.Join(r.Path,path))
  if e!=nil||rel=="."||!filepath.IsLocal(rel){continue}
  if len(result)>=decorationLimit{return nil,errors.New("too many SQL file decorations")}
  result=append(result,sqlDecoration{Path:filepath.Join(filepath.Clean(p.Path),rel),Badge:badge})
 }
 return result,nil
}
