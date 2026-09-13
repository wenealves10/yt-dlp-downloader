import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "./useAuth";
import {
  createIntegration,
  createKey,
  createWebhook,
  deleteIntegration,
  deleteWebhook,
  getDeliveries,
  getIntegration,
  getIntegrationDownloads,
  getIntegrations,
  getKeys,
  getRequests,
  getTraffic,
  getWebhooks,
  retryDelivery,
  revokeKey,
  testWebhook,
  updateIntegration,
  updateWebhook,
  type DeliveryFilters,
  type IntegrationDownloadFilters,
  type IntegrationFilters,
  type RequestFilters,
} from "../api/integrationsApi";

const raiz = ["admin", "integrations"];

export function useIntegrations(filters: IntegrationFilters) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "list", filters],
    queryFn: getIntegrations(token || "", filters),
    enabled: !!token,
    // Mantém a página anterior na tela durante a troca, em vez de piscar uma
    // tabela vazia.
    placeholderData: (anterior) => anterior,
  });
}

export function useIntegration(id: string | undefined, windowHours: number) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "detail", id, windowHours],
    queryFn: getIntegration(token || "", id || "", windowHours),
    enabled: !!token && !!id,
    // A tela fica aberta enquanto o sistema integrado trabalha; sem isso os
    // números congelam no instante do carregamento.
    refetchInterval: 30_000,
  });
}

// invalidarIntegracoes derruba o cache de tudo que compartilha os mesmos
// números: mexer numa chave muda a listagem e também os contadores do detalhe.
function useInvalidar() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: raiz });
}

export function useCreateIntegration() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({
    mutationFn: createIntegration(token || ""),
    onSuccess: invalidar,
  });
}

export function useUpdateIntegration() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({
    mutationFn: updateIntegration(token || ""),
    onSuccess: invalidar,
  });
}

export function useDeleteIntegration() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({
    mutationFn: deleteIntegration(token || ""),
    onSuccess: invalidar,
  });
}

// ---------------------------------------------------------------------------
// Chaves
// ---------------------------------------------------------------------------

export function useIntegrationKeys(id: string | undefined) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "keys", id],
    queryFn: getKeys(token || "", id || ""),
    enabled: !!token && !!id,
  });
}

export function useCreateKey() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({ mutationFn: createKey(token || ""), onSuccess: invalidar });
}

export function useRevokeKey() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({ mutationFn: revokeKey(token || ""), onSuccess: invalidar });
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

export function useWebhooks(id: string | undefined) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "webhooks", id],
    queryFn: getWebhooks(token || "", id || ""),
    enabled: !!token && !!id,
  });
}

export function useCreateWebhook() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({ mutationFn: createWebhook(token || ""), onSuccess: invalidar });
}

export function useUpdateWebhook() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({ mutationFn: updateWebhook(token || ""), onSuccess: invalidar });
}

export function useDeleteWebhook() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({ mutationFn: deleteWebhook(token || ""), onSuccess: invalidar });
}

export function useTestWebhook() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({ mutationFn: testWebhook(token || ""), onSuccess: invalidar });
}

// ---------------------------------------------------------------------------
// Entregas, auditoria e downloads
// ---------------------------------------------------------------------------

export function useDeliveries(id: string | undefined, filters: DeliveryFilters) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "deliveries", id, filters],
    queryFn: getDeliveries(token || "", id || "", filters),
    enabled: !!token && !!id,
    placeholderData: (anterior) => anterior,
    // Uma entrega em reenvio muda de estado sozinha, em segundos.
    refetchInterval: 15_000,
  });
}

export function useRetryDelivery() {
  const { token } = useAuth();
  const invalidar = useInvalidar();
  return useMutation({ mutationFn: retryDelivery(token || ""), onSuccess: invalidar });
}

export function useIntegrationRequests(id: string | undefined, filters: RequestFilters) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "requests", id, filters],
    queryFn: getRequests(token || "", id || "", filters),
    enabled: !!token && !!id,
    placeholderData: (anterior) => anterior,
  });
}

export function useIntegrationTraffic(id: string | undefined, windowHours: number) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "traffic", id, windowHours],
    queryFn: getTraffic(token || "", id || "", windowHours),
    enabled: !!token && !!id,
    refetchInterval: 60_000,
  });
}

export function useIntegrationDownloads(
  id: string | undefined,
  filters: IntegrationDownloadFilters
) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "downloads", id, filters],
    queryFn: getIntegrationDownloads(token || "", id || "", filters),
    enabled: !!token && !!id,
    placeholderData: (anterior) => anterior,
    // Downloads em andamento avançam enquanto a tela está aberta.
    refetchInterval: 20_000,
  });
}
