import { Button, Card, P } from '@cate/ui';
import { t } from 'i18next';
import { useState } from 'react';
import { useAssignModelToPool, usePools } from '../../../../hooks/usePool';
import { Model } from '../../../../lib/types';

type ModelCardProps = {
  model: Model;
  onDelete: (model: string) => void;
  deletePending: boolean;
};

export function ModelCard({ model, onDelete, deletePending }: ModelCardProps) {
  const { data: pools } = usePools();
  const assignMutation = useAssignModelToPool();
  const [selectedPool, setSelectedPool] = useState('');

  const handleAssign = (poolID: string) => {
    setSelectedPool(poolID);
    assignMutation.mutate({ poolID, modelID: model.id });
  };

  return (
    <Card key={model.model} className="flex flex-col gap-2 p-4">
      <div className="flex justify-between">
        <div>
          <P variant="cardTitle">{model.model}</P>
          {model.createdAt && (
            <P>
              {t('common.created_at')} {model.createdAt}
            </P>
          )}
          {model.updatedAt && (
            <P>
              {t('common.updated_at')} {model.updatedAt}
            </P>
          )}
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onDelete(model.model)}
          className="text-error"
          disabled={deletePending}>
          {deletePending ? t('common.deleting') : t('translation:model.model_delete')}
        </Button>
      </div>

      <div className="flex items-center gap-2">
        <label htmlFor={`assign-${model.model}`} className="text-sm">
          {t('model.assign_to_pool')}
        </label>
        <select
          id={`assign-${model.model}`}
          className="rounded border px-2 py-1 text-sm"
          value={selectedPool}
          onChange={e => handleAssign(e.target.value)}
          disabled={assignMutation.isPending}>
          <option value="">{t('model.select_pool')}</option>
          {pools?.map(pool => (
            <option key={pool.id} value={pool.id}>
              {pool.name}
            </option>
          ))}
        </select>
      </div>
    </Card>
  );
}
