import type { Metadata } from "next";
import { OrgSelectionView } from "@/features/orgs/OrgSelectionView";

export const metadata: Metadata = {
  title: "Select organization",
};

export default function SelectOrgPage() {
  return <OrgSelectionView />;
}
