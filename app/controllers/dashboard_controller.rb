class DashboardController < ApplicationController
  before_action :set_date_range
  before_action :set_filters

  def index
    session_events = filtered_session_events.to_a
    all_events = filtered_events.to_a

    @stats = {
      total_events: filtered_events.count,
      coding_time: calculate_coding_time(all_events.sort_by(&:time)),
      session_time: sum_timing(session_events, :session_duration_seconds),
      agent_time: sum_timing(session_events, :agent_active_seconds),
      human_time: sum_timing(session_events, :human_active_seconds),
      active_projects: filtered_events.distinct.count(:project_id),
      total_tokens: filtered_events.sum(:tokens_used).to_i,
      avg_session_length: calculate_avg_session_length(session_events),
      productivity_ratio: calculate_productivity_ratio(session_events)
    }

    @top_project = top_project
    @peak_hour = peak_coding_hour(all_events)

    @recent_heartbeats = filtered_events
                          .order(time: :desc)
                          .limit(50)
                          .includes(:project)

    @language_breakdown = language_breakdown
    @agent_breakdown = agent_breakdown
    @daily_activity = build_daily_activity
    @daily_session_timing = build_daily_session_timing
    @hourly_activity = build_hourly_activity(all_events)
    @tokens_by_day = build_tokens_by_day

    @available_projects = current_user.projects.order(:name).pluck(:name, :id)
    @available_languages = current_user.heartbeat_events.where.not(language: nil).distinct.pluck(:language).sort
  end

  private

  def set_date_range
    @date_range = params[:date_range] || "today"
    @start_date, @end_date = parse_date_range(@date_range)
  end

  def set_filters
    @selected_project = params[:project_id].presence
    @selected_agent = params[:agent_type].presence
    @selected_language = params[:language].presence
  end

  def parse_date_range(range)
    case range
    when "today"
      [Time.zone.now.beginning_of_day, Time.zone.now.end_of_day]
    when "yesterday"
      [1.day.ago.beginning_of_day, 1.day.ago.end_of_day]
    when "last_7_days"
      [6.days.ago.beginning_of_day, Time.zone.now.end_of_day]
    when "last_30_days"
      [29.days.ago.beginning_of_day, Time.zone.now.end_of_day]
    when "custom"
      [
        parse_custom_date(params[:start_date])&.beginning_of_day || Time.zone.now.beginning_of_day,
        parse_custom_date(params[:end_date])&.end_of_day || Time.zone.now.end_of_day
      ]
    else
      [Time.zone.now.beginning_of_day, Time.zone.now.end_of_day]
    end
  end

  def parse_custom_date(date_string)
    return nil if date_string.blank?
    Date.parse(date_string)
  rescue ArgumentError
    nil
  end

  def filtered_events
    scope = current_user.heartbeat_events.for_period(@start_date, @end_date)
    scope = scope.where(project_id: @selected_project) if @selected_project.present?
    scope = scope.where(agent_type: @selected_agent) if @selected_agent.present?
    scope = scope.where(language: @selected_language) if @selected_language.present?
    scope
  end

  def filtered_session_events
    filtered_events.with_session_timing
  end

  def calculate_coding_time(events)
    return 0 if events.empty?

    total = 0
    events.each_cons(2) do |a, b|
      diff = b.time - a.time
      total += diff if diff <= 120
    end
    total.to_i
  end

  def calculate_avg_session_length(events)
    return 0 if events.empty?
    total = events.sum { |e| e.session_duration_seconds.to_i }
    (total / events.count.to_f).round
  end

  def calculate_productivity_ratio(events)
    return 0 if events.empty?
    agent_time = events.sum { |e| e.agent_active_seconds.to_i }
    human_time = events.sum { |e| e.human_active_seconds.to_i }
    total = agent_time + human_time
    return 0 if total == 0
    ((agent_time.to_f / total) * 100).round
  end

  def top_project
    filtered_events
      .where.not(project_id: nil)
      .group(:project_id)
      .count
      .max_by { |_, count| count }
      &.then { |project_id, count| [current_user.projects.find_by(id: project_id)&.name, count] }
  end

  def peak_coding_hour(events)
    return nil if events.empty?
    events.group_by { |e| Time.at(e.time).hour }.max_by { |_, evts| evts.count }&.first
  end

  def language_breakdown
    filtered_events.group(:language)
                   .count
                   .reject { |k, _| k.blank? }
                   .sort_by { |_, v| -v }
                   .first(8)
  end

  def agent_breakdown
    filtered_events.group(:agent_type)
                   .count
                   .sort_by { |_, v| -v }
  end

  def build_daily_activity
    days = (@end_date.to_date - @start_date.to_date).to_i + 1
    days = [days, 30].min

    (days - 1).downto(0).map do |offset|
      date = @end_date.to_date - offset.days
      events = filtered_events
                 .for_period(date.beginning_of_day, date.end_of_day)
                 .order(time: :asc)
                 .to_a
      {
        date: date.iso8601,
        label: date.strftime(days > 7 ? "%b %d" : "%a"),
        seconds: calculate_coding_time(events),
        events: events.count
      }
    end
  end

  def build_daily_session_timing
    days = (@end_date.to_date - @start_date.to_date).to_i + 1
    days = [days, 30].min

    (days - 1).downto(0).map do |offset|
      date = @end_date.to_date - offset.days
      events = filtered_session_events
                 .for_period(date.beginning_of_day, date.end_of_day)
                 .to_a

      {
        date: date.iso8601,
        label: date.strftime(days > 7 ? "%b %d" : "%a"),
        session_seconds: sum_timing(events, :session_duration_seconds),
        agent_seconds: sum_timing(events, :agent_active_seconds),
        human_seconds: sum_timing(events, :human_active_seconds)
      }
    end
  end

  def build_hourly_activity(events)
    return [] if events.empty?
    hours = Array.new(24, 0)
    events.each do |event|
      hour = Time.at(event.time).hour
      hours[hour] += 1
    end
    hours.map.with_index do |count, hour|
      { hour: format("%02d:00", hour), count: count }
    end
  end

  def build_tokens_by_day
    days = (@end_date.to_date - @start_date.to_date).to_i + 1
    days = [days, 30].min

    (days - 1).downto(0).map do |offset|
      date = @end_date.to_date - offset.days
      scope = filtered_events.for_period(date.beginning_of_day, date.end_of_day)

      {
        date: date.strftime(days > 7 ? "%b %d" : "%a"),
        tokens: scope.sum(:tokens_used).to_i
      }
    end
  end

  def sum_timing(events, field)
    Array(events).sum { |event| event.public_send(field).to_i }
  end

  def format_duration(seconds)
    hours = seconds / 3600
    mins = (seconds % 3600) / 60
    return "0 min" if hours == 0 && mins == 0
    parts = []
    parts << "#{hours}h" if hours > 0
    parts << "#{mins}m" if mins > 0
    parts.join(" ")
  end
  helper_method :format_duration
end
