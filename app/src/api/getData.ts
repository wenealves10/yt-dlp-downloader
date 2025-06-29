import type { DownloadsResponse } from "../interface/Download";

const apiUrl = import.meta.env.VITE_API_URL;

export function getData(token: string) {
  return async (perPage: number, page: number): Promise<DownloadsResponse> => {
    const res = await fetch(
      `${apiUrl}/v1/downloads?perPage=${perPage}&page=${page}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!res.ok) {
      throw new Error("Erro ao buscar downloads");
    }

    return res.json();
  };
}
