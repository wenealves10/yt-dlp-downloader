export interface User {
  id: string;
  full_name: string;
  photo_url: string; // URL da imagem de perfil
  email: string;
  password_changed_at: string; // ou Date se você for converter
  plan: "free" | "premium" | "enterprise";
  role: "user" | "admin" | "super_admin";
  daily_limit: number;
  is_verified: boolean;
  created_at: string; // ou Date
}
