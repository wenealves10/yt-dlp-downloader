import { Navigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { useQuery } from "@tanstack/react-query";
import type React from "react";
import Loading from "../loading/Loading";

export default function PrivateRoute({
  children,
}: {
  children: React.ReactNode;
}) {
  const { token, setUser } = useAuth();
  const { data, isLoading, error } = useQuery({
    queryKey: ["me"],
    queryFn: async () => {
      const res = await fetch("http://localhost:8080/v1/profile", {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        method: "GET",
      });
      if (!res.ok) throw new Error("Not authenticated");
      return res.json();
    },
  });

  if (data) {
    setUser(data);
  }

  if (isLoading) <Loading />;
  if (error) return <Navigate to="/" />;

  return children;
}
