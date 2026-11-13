import { expect, it } from 'vitest';
import { doctorFromChecks, doctorWithUpdatePolicy, withStreamedCheck } from './doctorUpdates';
import type { DoctorResponse } from '../api/client';

it('hides cached update notices without hiding diagnostic warnings or miscounting them', () => {
 const data: DoctorResponse = {
  checks: [
   {name:'skill_version',status:'warning',message:'Skill update',update_available:true},
   {name:'skill_version',status:'warning',message:'Skill not found'},
  ],
  summary:{total:2,pass:0,errors:0,warnings:2,info:0},
  version:{current:'1.0.0',latest:'9.9.9',update_available:true},
 };
 const quiet=doctorWithUpdatePolicy(data,false);
 expect(quiet.checks).toEqual([data.checks[1]]);
 expect(quiet.summary).toEqual({total:1,pass:0,errors:0,warnings:1,info:0});
 expect(quiet.version).toEqual({current:'1.0.0',update_available:false});
 expect(doctorWithUpdatePolicy(data,true)).toBe(data);
});

it('counts the checks found so far the way doctor does', () => {
 const partial=doctorFromChecks([
  {name:'source',status:'pass',message:'ok'},
  {name:'skillignore',status:'info',message:'none'},
  {name:'git_status',status:'warning',message:'dirty'},
  {name:'targets',status:'error',message:'missing'},
 ]);
 expect(partial.summary).toEqual({total:4,pass:1,warnings:1,errors:1,info:1});
});

it('puts a pending check result in its row and keeps the checks streamed after it', () => {
 let checks=withStreamedCheck([], {name:'plugins',status:'pending',message:'Checking plugins...'});
 checks=withStreamedCheck(checks, {name:'backup',status:'pass',message:'ok'});
 checks=withStreamedCheck(checks, {name:'plugins',status:'warning',message:'Plugins: 1 warning(s)'});
 expect(checks.map((c)=>`${c.name}:${c.status}`)).toEqual(['plugins:warning','backup:pass']);
});
