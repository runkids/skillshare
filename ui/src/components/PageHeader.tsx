interface PageHeaderProps {
  title: string;
  subtitle?: React.ReactNode;
  icon: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}

export default function PageHeader({ title, subtitle, icon, actions, className = '' }: PageHeaderProps) {
  const heading = (
    <div>
      <h2 className="text-2xl md:text-3xl font-bold text-pencil flex items-center gap-2">
        {icon}
        {title}
      </h2>
      {subtitle && <p className="text-pencil-light mt-1">{subtitle}</p>}
    </div>
  );

  return (
    <div
      className={`mb-6 ${actions ? 'flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4' : ''} ${className}`.trim()}
    >
      {heading}
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  );
}
