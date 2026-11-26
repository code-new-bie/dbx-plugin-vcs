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
