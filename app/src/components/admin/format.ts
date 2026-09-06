// Formatadores compartilhados pelas telas administrativas. Ficam juntos porque
// a mesma unidade precisa aparecer igual no gráfico, no card e na tabela.

const UNIDADES = ["B", "KB", "MB", "GB", "TB", "PB"];

// formatBytes usa base 1024, que é como o sistema de arquivos e o painel do R2
// reportam. Números grandes ganham menos casas: "1.2 TB" lê melhor que
// "1.23 TB" num card.
export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "0 B";

  const indice = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    UNIDADES.length - 1
  );
  const valor = bytes / Math.pow(1024, indice);
  const casas = valor >= 100 || indice === 0 ? 0 : valor >= 10 ? 1 : 2;

  return `${valor.toFixed(casas)} ${UNIDADES[indice]}`;
}

export function formatNumber(valor: number): string {
  return new Intl.NumberFormat("pt-BR").format(valor ?? 0);
}

export function formatDateTime(valor?: string): string {
  if (!valor) return "—";
  const data = new Date(valor);
  if (Number.isNaN(data.getTime())) return "—";
  return data.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}

export function formatDate(valor?: string): string {
  if (!valor) return "—";
  const data = new Date(valor);
  if (Number.isNaN(data.getTime())) return "—";
  return data.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" });
}

// rotuloBucket adapta o eixo à granularidade escolhida: mostrar a hora num
// intervalo de 90 dias, ou só o dia num intervalo de 24 horas, deixa o eixo
// ilegível ou ambíguo.
export function rotuloBucket(valor: string, granularity: string): string {
  const data = new Date(valor);
  if (Number.isNaN(data.getTime())) return "";

  switch (granularity) {
    case "hour":
      return data.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
    case "month":
      return data.toLocaleDateString("pt-BR", { month: "short", year: "2-digit" });
    default:
      return data.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" });
  }
}

export function formatDuration(segundos: number): string {
  if (!segundos || segundos <= 0) return "—";
  const horas = Math.floor(segundos / 3600);
  const minutos = Math.floor((segundos % 3600) / 60);
  const resto = Math.floor(segundos % 60);
  if (horas > 0) {
    return `${horas}h${String(minutos).padStart(2, "0")}`;
  }
  return `${minutos}:${String(resto).padStart(2, "0")}`;
}
