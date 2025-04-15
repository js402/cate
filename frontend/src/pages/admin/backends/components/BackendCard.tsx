import { Button, ButtonGroup, P, Section } from '@cate/ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAssignBackendToPool, usePools, usePoolsForBackend } from '../../../../hooks/usePool';
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

  const { data: pools } = usePools();
  const { data: backendPools } = usePoolsForBackend(backend.id);
  const assignMutation = useAssignBackendToPool();

  const currentPoolId = backendPools?.[0]?.id ?? '';

  const [selectedPool, setSelectedPool] = useState<string>(currentPoolId);

  const handleDelete = async (id: string) => {
    setDeletingBackendId(id);
    try {
      await onDelete(id);
    } finally {
      setDeletingBackendId(null);
    }
  };

  const handleAssignPool = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const poolId = e.target.value;
    setSelectedPool(poolId);
    if (poolId) {
      assignMutation.mutate({ poolID: poolId, backendID: backend.id });
    }
  };

  const getDownloadStatusForModel = (
    statusMap: Record<string, DownloadStatus>,
    baseUrl: string,
    modelName: string,
  ): DownloadStatus | undefined => {
    const key = `${baseUrl}:${modelName}`;
    return statusMap[key];
  };

  return (
    <Section title={backend.name} key={backend.id}>
      <P>{backend.baseUrl}</P>
      <P>
        {t('common.type')} {backend.type}
      </P>

      {backend.models?.map(model => (
        <ModelStatusDisplay
          key={model}
          modelName={model}
          downloadStatus={getDownloadStatusForModel(statusMap, backend.baseUrl, model)}
          isPulled={false}
        />
      ))}

      <label className="mt-4 block text-sm font-medium">{t('backends.assign_to_pool')}</label>
      <select
        className="mt-1 w-full rounded border p-2"
        value={selectedPool}
        onChange={handleAssignPool}>
        <option value="">{t('backends.select_pool')}</option>
        {pools.map(pool => (
          <option key={pool.id} value={pool.id}>
            {pool.name}
          </option>
        ))}
      </select>

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
