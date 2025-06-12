import { Link } from 'react-router-dom';
import { Briefcase } from 'lucide-react';

export function Header() {
  return (
    <header className="border-b bg-white">
      <div className="container mx-auto px-4">
        <div className="flex h-16 items-center justify-between">
          <Link to="/" className="flex items-center space-x-2">
            <Briefcase className="h-6 w-6 text-primary-600" />
            <span className="text-xl font-bold">TaskQ</span>
          </Link>
          
          <nav className="flex items-center space-x-6">
            <Link to="/" className="text-sm font-medium hover:text-primary-600">
              Home
            </Link>
            <Link to="/jobs" className="text-sm font-medium hover:text-primary-600">
              Jobs
            </Link>
            <a
              href="https://github.com/Devashish08/TaskQ"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm font-medium hover:text-primary-600"
            >
              GitHub
            </a>
          </nav>
        </div>
      </div>
    </header>
  );
}