-- A ordem importa: as tabelas dependentes saem antes de `integrations`, e o
-- tipo enum só pode cair depois da coluna que o usa.
DROP TABLE IF EXISTS "integration_requests";
DROP TABLE IF EXISTS "integration_webhook_deliveries";
DROP TYPE IF EXISTS "core"."webhook_delivery_status";
DROP TABLE IF EXISTS "integration_webhooks";
DROP TABLE IF EXISTS "integration_api_keys";

-- As contas de serviço ficam: elas são referenciadas pelo histórico de
-- downloads, e apagá-las levaria junto a contabilidade de armazenamento — o
-- mesmo motivo pelo qual a remoção de usuário no painel é lógica.
DROP TABLE IF EXISTS "integrations";

ALTER TABLE "users" DROP COLUMN IF EXISTS "kind";
DROP TYPE IF EXISTS "core"."user_kind";
