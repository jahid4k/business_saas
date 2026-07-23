import React, { useEffect, useState } from 'react';
import { View, Text, StyleSheet, ScrollView, ActivityIndicator, TouchableOpacity } from 'react-native';
import { useTheme } from '@/theme/ThemeProvider';
import { ChevronDown, Plus, Check } from 'lucide-react-native';
import { tasksApi, Task, TaskStatus } from '@/lib/tasksApi';
import { useLocalSearchParams } from 'expo-router';

function formatDate(isoString?: string) {
  if (!isoString) return '';
  const d = new Date(isoString);
  const month = d.toLocaleString('default', { month: 'short' });
  return `${month} ${d.getDate()}`;
}

const StatusPill = ({ status }: { status: TaskStatus }) => {
  const theme = useTheme();
  switch (status) {
    case 'todo':
      return (
        <View style={[styles.pill, { backgroundColor: theme.bgSurface, borderColor: theme.border, borderWidth: 1 }]}>
          <Text style={[styles.pillText, { color: theme.textMuted }]}>To Do</Text>
        </View>
      );
    case 'in_progress':
      return (
        <View style={[styles.pill, { backgroundColor: '#fef3c7' }]}>
          <Text style={[styles.pillText, { color: '#b45309' }]}>In Progress</Text>
        </View>
      );
    case 'done':
      return (
        <View style={[styles.pill, { backgroundColor: '#d1fae5' }]}>
          <Text style={[styles.pillText, { color: '#047857' }]}>Done</Text>
        </View>
      );
    default:
      return null;
  }
};

export default function TasksScreen() {
  const theme = useTheme();
  const { orgId } = useLocalSearchParams<{ orgId: string }>();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (orgId) {
      tasksApi.listTasks(orgId)
        .then(res => setTasks(res.tasks || []))
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [orgId]);

  const todoTasks = tasks.filter(t => t.status === 'todo');
  const inProgressTasks = tasks.filter(t => t.status === 'in_progress');
  const doneTasks = tasks.filter(t => t.status === 'done');

  const renderSection = (title: string, count: number, dotColor: string, data: Task[], isDone: boolean = false) => (
    <View style={styles.section}>
      <View style={styles.sectionHeader}>
        <View style={styles.sectionHeaderLeft}>
          <View style={[styles.dot, { backgroundColor: dotColor }]} />
          <Text style={[styles.sectionTitle, { color: theme.textPrimary }]}>{title}</Text>
          <Text style={[styles.sectionCount, { color: theme.textMuted }]}>{count}</Text>
        </View>
        <ChevronDown size={16} color={theme.textMuted} />
      </View>
      
      {data.map(task => (
        <View key={task.id} style={[styles.taskItem, { borderBottomColor: theme.border }]}>
          <View style={styles.taskContent}>
            <Text style={[styles.taskTitle, { color: isDone ? theme.textMuted : theme.textPrimary, textDecorationLine: isDone ? 'line-through' : 'none' }]}>
              {task.title}
            </Text>
            {task.dueDate && (
              <Text style={[styles.taskDueDate, { color: theme.textMuted }]}>
                {isDone ? 'Completed' : 'Due'} {formatDate(task.dueDate)}
              </Text>
            )}
          </View>
          <StatusPill status={task.status} />
        </View>
      ))}
      
      {!isDone && (
        <TouchableOpacity style={styles.newTaskBtn}>
          <Plus size={16} color={theme.accent} />
          <Text style={[styles.newTaskText, { color: theme.accent }]}>New task</Text>
        </TouchableOpacity>
      )}
    </View>
  );

  return (
    <View style={[styles.container, { backgroundColor: theme.bgCanvas }]}>
      <View style={styles.header}>
        <Text style={[styles.headerTitle, { color: theme.textPrimary }]}>Tasks</Text>
      </View>

      {loading ? (
        <ActivityIndicator size="large" color={theme.accent} style={{ marginTop: 60 }} />
      ) : (
        <ScrollView contentContainerStyle={styles.scrollContent}>
          {renderSection('TO DO', todoTasks.length, '#94a3b8', todoTasks)}
          {renderSection('IN PROGRESS', inProgressTasks.length, '#f59e0b', inProgressTasks)}
          {renderSection('DONE', doneTasks.length, '#10b981', doneTasks, true)}
        </ScrollView>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  header: {
    paddingHorizontal: 24,
    paddingTop: 60,
    paddingBottom: 24,
  },
  headerTitle: {
    fontSize: 28,
    fontWeight: 'bold',
  },
  scrollContent: {
    paddingHorizontal: 24,
    paddingBottom: 40,
  },
  section: {
    marginBottom: 32,
  },
  sectionHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 16,
  },
  sectionHeaderLeft: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  dot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  sectionTitle: {
    fontSize: 13,
    fontWeight: 'bold',
    letterSpacing: 1,
  },
  sectionCount: {
    fontSize: 13,
  },
  taskItem: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 16,
    borderBottomWidth: 1,
  },
  taskContent: {
    flex: 1,
    paddingRight: 16,
  },
  taskTitle: {
    fontSize: 15,
    fontWeight: '600',
    marginBottom: 4,
  },
  taskDueDate: {
    fontSize: 13,
  },
  pill: {
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 12,
  },
  pillText: {
    fontSize: 11,
    fontWeight: 'bold',
  },
  newTaskBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    paddingVertical: 16,
  },
  newTaskText: {
    fontSize: 15,
    fontWeight: '500',
  }
});
