import { expect, it } from 'vitest';
import { doctorWithUpdatePolicy } from './doctorUpdates';
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
