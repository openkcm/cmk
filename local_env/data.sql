DELETE FROM tenant1.keys;
DELETE FROM tenant1.key_versions;
DELETE FROM tenant1.systems;
DELETE FROM tenant1.systems_properties;
DELETE FROM tenant1.key_configurations;

INSERT INTO tenant1.systems values('71e7a444-1a81-4b82-8565-1ed797b23d22', 'e47fdacf-cb4e-48aa-b48c-cad67795b7e9', 'eu10', 'SUBACCOUNT', NULL, 'DISCONNECTED');
INSERT INTO tenant1.systems_properties values('71e7a444-1a81-4b82-8565-1ed797b23d22', 'name', 'Name');
INSERT INTO tenant1.systems_properties values('71e7a444-1a81-4b82-8565-1ed797b23d22', 'systemrole', 'Role');

INSERT INTO tenant1.systems values('1820b95d-a4f0-4d44-ab4c-cce965becace', 'e47fdacf-cb4e-48aa-b48c-cad67795b7e8', 'eu10-canary', 'SUBACCOUNT', NULL, 'DISCONNECTED');
INSERT INTO tenant1.systems_properties values('1820b95d-a4f0-4d44-ab4c-cce965becace', 'systemrole', 'Role');

INSERT INTO tenant1.systems values('f26714d5-e248-4eec-8bf1-26da486322b4', 'e47fdacf-cb4e-48aa-b48c-cad67795b7e7', 'eu10-canary', 'SUBACCOUNT', NULL, 'DISCONNECTED');
INSERT INTO tenant1.systems_properties values('f26714d5-e248-4eec-8bf1-26da486322b4', 'name', 'Name');

INSERT INTO tenant1.systems values('6f18959d-f377-4892-9d5d-5e286b52b960', 'e47fdacf-cb4e-48aa-b48c-cad67795b7e6', 'eu20', 'SUBACCOUNT', NULL, 'DISCONNECTED');

INSERT INTO tenant1.systems values('5978f83a-447b-4773-a744-5387be32dc8f', 'e47fdacf-cb4e-48aa-b48c-cad67795b7e5', 'eu20', 'SUBACCOUNT', NULL, 'DISCONNECTED');
