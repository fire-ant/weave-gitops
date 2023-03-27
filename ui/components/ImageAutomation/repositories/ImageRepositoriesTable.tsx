import React from "react";
import { useListImageAutomation } from "../../../hooks/imageautomation";
import { Kind } from "../../../lib/api/core/types.pb";
import { formatURL } from "../../../lib/nav";
import { showInterval } from "../../../lib/time";
import { Source, V2Routes } from "../../../lib/types";
import DataTable, { filterConfig } from "../../DataTable";
import KubeStatusIndicator from "../../KubeStatusIndicator";
import Link from "../../Link";
import RequestStateHandler from "../../RequestStateHandler";

const ImageRepositoriesTable = () => {
  const { data, isLoading, error } = useListImageAutomation(
    Kind.ImageRepository
  );
  const initialFilterState = {
    ...filterConfig(data?.objects, "name"),
  };
  return (
    <RequestStateHandler loading={isLoading} error={error}>
      <DataTable
        filters={initialFilterState}
        rows={data?.objects}
        fields={[
          {
            label: "Name",
            value: ({ name, namespace, clusterName }) => (
              <Link
                to={formatURL(V2Routes.ImageAutomationRepositoryDetails, {
                  name: name,
                  namespace: namespace,
                  clusterName: clusterName,
                })}
              >
                {name}
              </Link>
            ),
            textSearchable: true,
            sortValue: ({ name }) => name || "",
            maxWidth: 600,
          },
          {
            label: "Namespace",
            value: "namespace",
          },
          {
            label: "Status",
            value: (s: Source) => (
              <KubeStatusIndicator
                short
                conditions={s.conditions}
                suspended={s.suspended}
              />
            ),
            defaultSort: true,
          },
          {
            label: "Interval",
            value: (s: Source) => showInterval(s.interval),
          },
          {
            label: "Tag Count",
            value: "tagCount",
          },
        ]}
      />
    </RequestStateHandler>
  );
};

export default ImageRepositoriesTable;
