// Formatadores da tela de download. Ficam juntos para o card, a barra de
// progresso e a lista de qualidade mostrarem a mesma unidade do mesmo jeito.

const UNIDADES = ["B", "KB", "MB", "GB", "TB"];

export function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return "—";

  const indice = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    UNIDADES.length - 1
  );
  const valor = bytes / Math.pow(1024, indice);
  const casas = valor >= 100 || indice === 0 ? 0 : 1;

  return `${valor.toFixed(casas)} ${UNIDADES[indice]}`;
}

export function formatSpeed(bytesPorSegundo?: number): string {
  if (!bytesPorSegundo || bytesPorSegundo <= 0) return "";
  return `${formatBytes(bytesPorSegundo)}/s`;
}

// formatETA usa "mm:ss" porque é como tempo restante se lê: ninguém pensa em
// "134 segundos".
export function formatETA(segundos?: number): string {
  if (!segundos || segundos <= 0) return "";

  const horas = Math.floor(segundos / 3600);
  const minutos = Math.floor((segundos % 3600) / 60);
  const resto = Math.floor(segundos % 60);

  if (horas > 0) {
    return `${horas}:${String(minutos).padStart(2, "0")}:${String(resto).padStart(2, "0")}`;
  }
  return `${minutos}:${String(resto).padStart(2, "0")}`;
}

export function formatDuration(segundos?: number): string {
  if (!segundos || segundos <= 0) return "";
  return formatETA(segundos);
}

// formatCompact encurta números grandes do jeito que se lê em português:
// "412 mi", e não "412.000.000". Intl faz o trabalho e respeita o idioma.
export function formatCompact(valor?: number): string {
  if (!valor || valor <= 0) return "";
  return new Intl.NumberFormat("pt-BR", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(valor);
}

// formatUploadDate recebe AAAA-MM-DD do servidor. Datas do mesmo ano omitem o
// ano, que é ruído quando o vídeo é recente.
export function formatUploadDate(iso?: string): string {
  if (!iso) return "";

  const data = new Date(`${iso}T12:00:00`);
  if (Number.isNaN(data.getTime())) return "";

  const mesmoAno = data.getFullYear() === new Date().getFullYear();
  return data.toLocaleDateString("pt-BR", {
    day: "numeric",
    month: "short",
    ...(mesmoAno ? {} : { year: "numeric" }),
  });
}

// resolverThumbnail lida com as duas origens possíveis. Enquanto o download
// acontece, a miniatura é a URL da própria plataforma; depois do upload, vira
// um caminho dentro do bucket. Tratar as duas como caminho fazia a imagem
// desaparecer justamente durante o download, que é quando o card mais precisa
// dela.
export function resolverThumbnail(
  valor: string | undefined,
  bucketHost: string
): string {
  if (!valor) return "";
  if (/^https?:\/\//i.test(valor)) return valor;
  return `${bucketHost}/${valor}`;
}
