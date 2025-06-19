export type LoginPayload = {
  email: string;
  password: string;
};

export async function login(payload: LoginPayload) {
  const res = await fetch("http://localhost:8080/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    throw new Error("Credenciais inválidas");
  }

  return res.json(); // deve retornar { access_token: "...", user: { ... } }
}

export type RegisterPayload = {
  full_name: string;
  email: string;
  password: string;
};

export async function register(payload: RegisterPayload) {
  const res = await fetch("http://localhost:8080/v1/auth/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    throw new Error("Erro ao registrar usuário");
  }

  return res.json();
}
