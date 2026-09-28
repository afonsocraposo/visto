import type { ReactNode } from "react";
import { Skeleton, Table } from "@mantine/core";

export function TableSkeleton({
  columns,
  rows = 5,
  renderCell,
}: {
  columns: string[];
  rows?: number;
  renderCell?: (columnIndex: number) => ReactNode;
}) {
  return (
    <Table>
      <Table.Thead>
        <Table.Tr>
          {columns.map((label, i) => (
            <Table.Th key={i}>{label}</Table.Th>
          ))}
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {Array.from({ length: rows }).map((_, rowIndex) => (
          <Table.Tr key={rowIndex}>
            {columns.map((_, columnIndex) => (
              <Table.Td key={columnIndex}>
                {renderCell ? renderCell(columnIndex) : <Skeleton height={12} width="70%" />}
              </Table.Td>
            ))}
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
