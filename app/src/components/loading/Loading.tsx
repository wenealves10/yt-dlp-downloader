export default function Loading() {
  return (
    <div className="flex items-center justify-center h-screen bg-white dark:bg-gray-900">
      <div className="flex flex-col items-center space-y-4">
        <div className="relative">
          <div className="w-16 h-16 border-4 border-blue-500 border-t-transparent rounded-full animate-spin"></div>
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="text-blue-500 font-bold text-sm">Carregando</span>
          </div>
        </div>
        <p className="text-gray-600 dark:text-gray-300 text-sm">
          Aguarde um momento...
        </p>
      </div>
    </div>
  );
}
