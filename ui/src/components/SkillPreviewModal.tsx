import { useState, useEffect, useCallback } from 'react';
import { X, Star, Download, RotateCw, Info } from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import DialogShell from './DialogShell';
import Button from './Button';
import Badge from './Badge';
import IconButton from './IconButton';
import Spinner from './Spinner';
import { api, type SkillPreview, type SearchResult } from '../api/client';
import { useT } from '../i18n';
import { parseSkillMarkdown } from '../lib/frontmatter';

interface SkillPreviewModalProps {
  result: SearchResult;
  onClose: () => void;
  onInstall: (source: string, skill?: string) => void;
  installing?: boolean;
}

export default function SkillPreviewModal({
  result,
  onClose,
  onInstall,
  installing = false,
}: SkillPreviewModalProps) {
  const t = useT();
  const [data, setData] = useState<SkillPreview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchPreview = useCallback(() => {
    setLoading(true);
    setError(null);
    api
      .preview(result.source)
      .then(setData)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [result.source]);

  useEffect(() => {
    fetchPreview();
  }, [fetchPreview]);

  const name = data?.name || result.source.split('/').pop() || result.source;
  const owner = data?.owner || result.owner;
  const stars = data?.stars ?? result.stars ?? 0;
  const description = data?.description || result.description;
  const license = data?.license;
  const tags = data?.tags || result.tags;

  const hasFallback = !!(result.description || result.tags?.length);

  const markdownBody = data?.content
    ? parseSkillMarkdown(data.content).body
    : '';

  return (
    <DialogShell
      open={true}
      onClose={onClose}
      maxWidth="4xl"
      padding="none"
      preventClose={installing}
      className="max-h-[85vh] flex flex-col overflow-hidden"
    >
      {/* Header */}
      <div className="flex items-center justify-between gap-3 px-6 pt-5 pb-3 border-b-2 border-dashed border-muted">
        <div className="flex items-center gap-2 min-w-0 flex-wrap">
          <h2 className="font-bold text-pencil text-lg truncate">{name}</h2>
          {stars > 0 && (
            <span className="flex items-center gap-1 text-sm text-warning shrink-0">
              <Star size={14} strokeWidth={2.5} fill="currentColor" />
              {stars}
            </span>
          )}
          {owner && <Badge>{owner}</Badge>}
        </div>
        <IconButton
          icon={<X size={16} strokeWidth={2.5} />}
          label={t('common.close')}
          size="md"
          onClick={onClose}
          className="shrink-0"
        />
      </div>

      {/* Content */}
      <div className="overflow-auto flex-1 min-h-0 px-6 py-4">
        {loading && (
          <div className="py-12 flex flex-col items-center gap-3">
            <Spinner size="md" />
            <p className="text-pencil-light text-sm">{t('preview.loading')}</p>
          </div>
        )}

        {error && !loading && !hasFallback && (
          <div className="py-8 text-center">
            <p className="text-danger mb-3">{t('preview.error')}</p>
            <p className="text-pencil-light text-sm mb-4">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchPreview}>
              <RotateCw size={14} strokeWidth={2.5} />
              {t('preview.retry')}
            </Button>
          </div>
        )}

        {(data || (error && hasFallback)) && !loading && (
          <>
            <div className="mb-4 space-y-2">
              {description && (
                <p className="text-pencil-light">{description}</p>
              )}
              <div className="flex flex-wrap items-center gap-2">
                {license && (
                  <Badge variant="default" size="sm">{license}</Badge>
                )}
                {tags && tags.length > 0 && tags.map((tag) => (
                  <Badge key={tag} variant="info" size="sm">#{tag}</Badge>
                ))}
              </div>
            </div>

            {markdownBody ? (
              <div className="prose-hand">
                <Markdown remarkPlugins={[remarkGfm]}>{markdownBody}</Markdown>
              </div>
            ) : (
              <div className="flex items-start gap-2 rounded border-2 border-dashed border-muted bg-surface px-3 py-2.5 text-sm text-pencil-light">
                <Info size={16} strokeWidth={2.5} className="mt-0.5 shrink-0 text-muted-dark" />
                <span>{t('preview.fallbackNotice')}</span>
              </div>
            )}
          </>
        )}
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between gap-3 px-6 py-3 border-t-2 border-dashed border-muted bg-surface">
        <p className="font-mono text-xs text-muted-dark truncate min-w-0">
          {result.source}
        </p>
        <Button
          onClick={() => onInstall(result.source, result.skill)}
          variant="primary"
          size="sm"
          loading={installing}
          disabled={loading || (!!error && !hasFallback)}
          className="shrink-0"
        >
          {!installing && <Download size={14} strokeWidth={2.5} />}
          {t('preview.install')}
        </Button>
      </div>
    </DialogShell>
  );
}
