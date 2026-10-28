-- Author: Sunil Jain, 2026-10-28.
-- F1.3: immutable-in-app chapter/action policy catalog. NO user access is granted.
-- Identity grants remain provisioner-owned workforce_identity_grants_v15;
-- table only defines the registered action-to-chapter mapping.
BEGIN;
CREATE TABLE workforce_chapter_actions_v17 (
 chapter_id smallint NOT NULL CHECK(chapter_id BETWEEN 1 AND 10),
 permission varchar(80) PRIMARY KEY CHECK(permission ~ '^[a-z][a-z0-9_.-]{2,79}$'),
 authority varchar(16) NOT NULL CHECK(authority IN ('READ','SIGNED_READ','SIGNED_WRITE')),
 UNIQUE(chapter_id,permission)
);
INSERT INTO workforce_chapter_actions_v17(chapter_id,permission,authority) VALUES
 (1,'factory.read','READ'),
 (1,'factory.manage','SIGNED_WRITE'),
 (2,'orders.read','READ'),
 (2,'orders.manage','SIGNED_WRITE'),
 (3,'planning.read','READ'),
 (3,'planning.approve','SIGNED_WRITE'),
 (4,'stores.read','READ'),
 (4,'stores.move','SIGNED_WRITE'),
 (5,'cutting.read','READ'),
 (5,'cutting.release','SIGNED_WRITE'),
 (6,'sewing.read','SIGNED_READ'),
 (6,'sewing.complete','SIGNED_WRITE'),
 (7,'quality.read','SIGNED_READ'),
 (7,'quality.inspect','SIGNED_WRITE'),
 (7,'quality.repair','SIGNED_WRITE'),
 (8,'packing.read','SIGNED_READ'),
 (8,'packing.stage','SIGNED_WRITE'),
 (9,'intelligence.read','READ'),
 (10,'documents.read','READ'),
 (10,'documents.manage','SIGNED_WRITE');
REVOKE ALL ON workforce_chapter_actions_v17 FROM PUBLIC,agiparel_app;
GRANT SELECT ON workforce_chapter_actions_v17 TO agiparel_app;
-- The production app has no INSERT/UPDATE/DELETE grant on either policy or workforce grants.
COMMIT;
