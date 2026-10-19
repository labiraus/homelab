-- Synthetic data for the owned disposable CNPG fixture only. Never a deployment migration.
BEGIN;
INSERT INTO worldbuilder.world(id,name,revision) VALUES ('00000000-0000-4000-8000-000000000001','Before base backup',1);
INSERT INTO worldbuilder.member(world_id,principal,role,authority_id) VALUES
 ('00000000-0000-4000-8000-000000000001','owner@example.test','author','00000000-0000-4000-8000-000000000002'),
 ('00000000-0000-4000-8000-000000000001','reviewer@example.test','author','00000000-0000-4000-8000-000000000003');
INSERT INTO worldbuilder.record(world_id,id,kind,label) VALUES
 ('00000000-0000-4000-8000-000000000001','main','timeline','Main'),
 ('00000000-0000-4000-8000-000000000001','person','type','Person'),
 ('00000000-0000-4000-8000-000000000001','notes','source','Fixture notes');
INSERT INTO worldbuilder.record(world_id,id,kind,label,type_id) VALUES
 ('00000000-0000-4000-8000-000000000001','hero','entity','Hero Ω','person');
INSERT INTO worldbuilder.record(world_id,id,kind,subject_id,timeline_id,source_id,predicate,value_type,value,valid_from,valid_until) VALUES
 ('00000000-0000-4000-8000-000000000001','description','fact','hero','main','notes','description','string','"First line\nSecond line Ω"','-9223372036854775808','9223372036854775807');
INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES
 ('00000000-0000-4000-8000-000000000001',0,'{}','owner@example.test','Fixture origin'),
 ('00000000-0000-4000-8000-000000000001',1,'{"main":{"id":"main","kind":"timeline","label":"Main"},"witness":{"id":"witness","kind":"established_account","label":"Witnessed collapse","timeline":"main","synopsis":"The party witnessed the bridge collapse Ω."},"witness-correction":{"id":"witness-correction","kind":"established_account","label":"Witnessed span collapse","timeline":"main","synopsis":"The party witnessed the central bridge span collapse Ω.","supersedes":"witness"}}','reviewer@example.test','Fixture history');
-- Capture the immutable ledger through revision insertion, never by seeding it directly.
INSERT INTO worldbuilder.record(world_id,id,kind,label,timeline_id) VALUES
 ('00000000-0000-4000-8000-000000000001','witness','established_account','Witnessed collapse','main'),
 ('00000000-0000-4000-8000-000000000001','witness-correction','established_account','Witnessed span collapse','main');
INSERT INTO worldbuilder.conversation(world_id,id,title,creator,revision,timeline,at) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000012','Shared witness account','owner@example.test',1,'main',-9223372036854775808);
INSERT INTO worldbuilder.conversation_message(world_id,conversation_id,id,actor,request,result,status) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000012','00000000-0000-4000-8000-000000000013','reviewer@example.test','{"workflow":"canon_query","question":"What was witnessed?","revision":1,"timeline":"main","at":-9223372036854775808}','{"answer":"The central span collapsed Ω.","citations":["00000000-0000-4000-8000-000000000001:1:main:witness-correction"]}','complete');
INSERT INTO worldbuilder.change_set(world_id,id,author,base_revision,reason,operations,status,reviewer,review_reason,published_revision,published_approvals,policy_version) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000004','owner@example.test',0,'Fixture publication','[]','published','reviewer@example.test','Independent fixture review',1,ARRAY['reviewer@example.test'],1);
INSERT INTO worldbuilder.change_review(world_id,change_id,reviewer,authority_id,decision,reason,policy_version) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000004','reviewer@example.test','00000000-0000-4000-8000-000000000003','approved','Independent fixture review',1);
INSERT INTO worldbuilder.audit(world_id,actor,action,resource,detail) VALUES
 ('00000000-0000-4000-8000-000000000001','reviewer@example.test','fixture.publish','fixture','{"message":"Preserve immutable history Ω"}');
INSERT INTO worldbuilder.outbox(id,world_id,revision,payload) VALUES
 ('00000000-0000-4000-8000-000000000005','00000000-0000-4000-8000-000000000001',1,'{"fixture":"pending canon relay"}');
INSERT INTO worldbuilder.story_definition(world_id,id,owner,pinned_revision,definition) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000006','owner@example.test',1,'{"fixture":"private definition"}');
INSERT INTO worldbuilder.story_session(world_id,id,owner,title,pinned_revision,timeline_id,version,definition_id,state,memory) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000007','owner@example.test','Private fixture',1,'main',1,'00000000-0000-4000-8000-000000000006','{"choice":"wait"}','{"summary":"fixture memory","throughVersion":1,"choices":["wait"]}');
INSERT INTO worldbuilder.story_turn(world_id,session_id,version,request_id,request,narration,state,memory) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000007',1,'00000000-0000-4000-8000-000000000008','{"choice":"wait"}','Synthetic narration Ω','{"choice":"wait"}','{"summary":"fixture memory","throughVersion":1,"choices":["wait"]}');
INSERT INTO worldbuilder.story_outcome(world_id,session_id,request_id,change_id,request) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000007','00000000-0000-4000-8000-000000000008','00000000-0000-4000-8000-000000000004','{"fixture":"reviewed outcome"}');
INSERT INTO worldbuilder.generation_job(world_id,id,owner,request,status,parts,step,results) VALUES
 ('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000009','owner@example.test','{"fixture":"pending generation"}','uncertain',2,1,'[{"fixture":"completed first part"}]');
INSERT INTO worldbuilder.generation_outbox(id,world_id,job_id,payload) VALUES
 ('00000000-0000-4000-8000-000000000010','00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000009','{"fixture":"pending generation relay"}');
INSERT INTO worldbuilder.inference_attempt(id,world_id,actor,workflow,revision,timeline,request_hash,lease_until,status,allocated_micros,currency,gateway_route,generation_job_id,generation_part) VALUES
 ('00000000-0000-4000-8000-000000000011','00000000-0000-4000-8000-000000000001','owner@example.test','fixture',1,'main','fixture-hash',now()+interval '1 hour','reserved',1000,'USD','fixture-route','00000000-0000-4000-8000-000000000009',2);
INSERT INTO auth.users(email,display_name) VALUES('owner@example.test','Fixture owner');
INSERT INTO worldbuilder.superuser(principal,granted_by) VALUES('owner@example.test','offline fixture operator');
COMMIT;
