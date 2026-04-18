interface CharBudgetProps {
  used: number;
  cap: number;
}

export default function CharBudget({ used, cap }: CharBudgetProps) {
  const tone =
    used > cap
      ? 'over'
      : used >= 1500
        ? 'amber'
        : used >= 1200
          ? 'warn'
          : 'ok';
  return (
    <span className={`char-budget tone-${tone}`} title="description + when_to_use combined">
      {used.toLocaleString()} / {cap.toLocaleString()}
    </span>
  );
}
