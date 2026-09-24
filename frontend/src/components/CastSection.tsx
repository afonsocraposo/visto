import { Image, Paper, SimpleGrid, Text, Title } from "@mantine/core";
import type { TVCastMember } from "../types";
import { posterURL } from "../lib/artwork";

type CastSectionProps = {
  members: TVCastMember[];
  title: string;
  kicker?: string;
  fallbackRole?: string;
};

export function CastSection({ members, title, kicker, fallbackRole = "Cast" }: CastSectionProps) {
  if (!members.length) return null;

  return <section className="detail-section">
    {kicker && <Text className="section-kicker">{kicker}</Text>}
    <Title order={2} mb="sm">{title}</Title>
    <SimpleGrid className="cast-grid" cols={{ base: 3, xs: 4, sm: 5, md: 6, lg: 8 }} spacing="sm">
      {members.slice(0, 12).map(member => <CastMemberCard key={`${member.id}-${member.character}`} member={member} fallbackRole={fallbackRole} />)}
    </SimpleGrid>
  </section>;
}

function CastMemberCard({ member, fallbackRole }: { member: TVCastMember; fallbackRole: string }) {
  const art = posterURL(member.profile_path, "w185");

  return <Paper className="cast-member-card" component="article" withBorder p={0}>
    <div className="cast-member-photo">
      {art ? <Image src={art} alt="" loading="lazy" /> : <div className="cast-member-fallback" aria-hidden="true">{member.name.slice(0, 1)}</div>}
    </div>
    <div className="cast-member-copy">
      <Text className="cast-member-name" fw={700} size="sm" lineClamp={2}>{member.name}</Text>
      <Text className="cast-member-character" size="xs" c="dimmed" lineClamp={2}>{member.character || fallbackRole}</Text>
    </div>
  </Paper>;
}
