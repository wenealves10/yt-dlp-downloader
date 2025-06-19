import { Link } from "react-router-dom";

export default function NotFound() {
  return (
    <div className="min-h-screen bg-[#0f172a] flex items-center justify-center px-4">
      <div className="w-full max-w-md p-8 bg-[#1e293b] rounded-lg shadow-lg text-center">
        <div className="flex justify-center mb-4">
          <svg
            className="w-10 h-10 text-red-600"
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zM11 6h2v6h-2V6zm0 8h2v2h-2v-2z" />
          </svg>
        </div>
        <h1 className="text-3xl font-bold text-white mb-2">
          Página não encontrada
        </h1>
        <p className="text-gray-300 mb-6">
          A URL que você acessou não existe ou foi removida.
        </p>
        <Link
          to="/"
          className="inline-block bg-red-600 hover:bg-red-700 text-white font-semibold py-2 px-4 rounded transition-all"
        >
          Voltar para o início
        </Link>
      </div>
    </div>
  );
}
