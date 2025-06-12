export function Footer() {
    return (
      <footer className="border-t bg-gray-50 py-8 mt-auto">
        <div className="container mx-auto px-4">
          <div className="text-center text-sm text-gray-600">
            <p>TaskQ - A Golang Job Queue System</p>
            <p className="mt-2">
              Built with Go, PostgreSQL, Redis, and React
            </p>
          </div>
        </div>
      </footer>
    );
  }