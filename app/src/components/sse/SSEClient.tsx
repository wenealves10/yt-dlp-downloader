import { useEffect, useState } from "react";
import { useAuth } from "../../hooks/useAuth";
const apiUrl = import.meta.env.VITE_API_URL;

const SSEClient = () => {
  const [messages, setMessages] = useState<string[]>([]);
  const { token } = useAuth();

  useEffect(() => {
    const eventSource = new EventSource(`${apiUrl}/v1/sse?token=${token}`);

    eventSource.onmessage = (event) => {
      console.log("Mensagem SSE:", event.data);
      setMessages((prev) => [...prev, event.data]);
    };

    eventSource.onerror = (err) => {
      console.error("Erro SSE:", err);
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }, []);

  return (
    <div>
      <h2>Mensagens SSE</h2>
      <ul>
        {messages.map((msg, i) => (
          <li key={i}>{msg}</li>
        ))}
      </ul>
    </div>
  );
};

export default SSEClient;
