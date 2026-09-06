-- Guarda a resolução pretendida do download, e não só o id do formato.
--
-- Os ids do yt-dlp NÃO são estáveis entre duas extrações do mesmo conteúdo:
-- em HLS eles carregam o CDN sorteado na hora, e o que /resolve ofereceu pode
-- não existir mais quando o worker vai baixar. Sem a altura, a única saída era
-- falhar com "o formato escolhido não está disponível" — um erro sobre a
-- escolha do usuário para um problema que não é dele.
--
-- Com ela o seletor degrada para a resolução mais próxima em vez de desistir.
-- Zero significa "não sei", e aí vale o melhor disponível.
ALTER TABLE "downloads"
  ADD COLUMN "format_height" INT NOT NULL DEFAULT 0;
