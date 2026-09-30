import { FileCog } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useT } from '../i18n';
import { useToast } from './Toast';
import Tooltip from './Tooltip';

/** Opens the source config in the Config page; a separate source file (sources.mcp) has no editor there, so a click copies its path. */
export default function SourcePathButton({ path, configPath }: { path: string; configPath: string }) {
  const t = useT();
  const { toast } = useToast();
  const label = <><FileCog size={15} />{path.split(/[\\/]/).pop()}</>;
  return (
    <Tooltip content={<><span className="font-sans">{t('mcp.source')}: </span><span className="font-mono">{path}</span></>}>
      {path === configPath
        ? <Link to="/config" className="ss-btn ghost">{label}</Link>
        : <button type="button" className="ss-btn ghost" aria-label={t('mcp.copySource')} onClick={() => { void navigator.clipboard?.writeText(path); toast(t('mcp.copied'), 'success'); }}>{label}</button>}
    </Tooltip>
  );
}
