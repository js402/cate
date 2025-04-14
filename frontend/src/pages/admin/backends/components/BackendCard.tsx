import { Button, ButtonGroup, P, Section } from '@cate/ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Backend, DownloadStatus } from '../../../../lib/types';
import { ModelStatusDisplay } from './ModelStatusDisplay';

type BackendCardProps = {
  backend: Backend;
  onEdit: (backend: Backend) => void;
  onDelete: (id: string) => Promise<void>;
  statusMap: Record<string, DownloadStatus>;
};

export function BackendCard({ backend, onEdit, onDelete, statusMap }: BackendCardProps) {
  const { t } = useTranslation();
  const [deletingBackendId, setDeletingBackendId] = useState<string | null>(null);
  const handleDelete = async (id: string) => {
    setDeletingBackendId(id);
    try {
      await onDelete(id);
    } finally {
      setDeletingBackendId(null);
    }
  };
  function getDownloadStatusForModel(
    statusMap: Record<string, DownloadStatus>,
    baseUrl: string,
    modelName: string,
  ): DownloadStatus | undefined {
    const key = `${baseUrl}:${modelName}`;
    return statusMap[key];
  }
  return (
    <Section title={backend.name} key={backend.id}>
      <P>{backend.baseUrl}</P>
      <P>
        {t('common.type')} {backend.type}
      </P>
      {backend.models?.map(model => (
        <ModelStatusDisplay
          modelName={model}
          downloadStatus={getDownloadStatusForModel(statusMap, backend.baseUrl, model)}
          isPulled={false}
        />
      ))}

      <ButtonGroup>
        <Button variant="ghost" size="sm" onClick={() => onEdit(backend)}>
          {t('common.edit')}
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => handleDelete(backend.id)}
          disabled={deletingBackendId === backend.id}>
          {deletingBackendId === backend.id ? t('common.deleting') : t('common.delete')}
        </Button>
      </ButtonGroup>
    </Section>
  );
}
