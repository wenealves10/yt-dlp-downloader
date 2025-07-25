const apiUrl = import.meta.env.VITE_API_URL;

export type LoginPayload = {
  email: string;
  password: string;
  turnstileToken: string;
};

export async function login(payload: LoginPayload) {
  const res = await fetch(`${apiUrl}/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    throw new Error("Credenciais inválidas");
  }

  return res.json();
}

export type RegisterPayload = {
  full_name: string;
  email: string;
  password: string;
  turnstileToken: string;
};

export async function register(payload: RegisterPayload) {
  const res = await fetch(`${apiUrl}/v1/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    throw new Error("Erro ao registrar usuário");
  }

  return res.json();
}

export async function getProfile(token: string) {
  const res = await fetch(`${apiUrl}/v1/profile`, {
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
  });
  if (!res.ok) {
    throw new Error("Erro ao obter perfil");
  }
  return res.json();
}
