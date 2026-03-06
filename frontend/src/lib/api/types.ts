export interface User {
	id: number;
	email: string;
	name: string;
	avatar_url?: string;
}

export interface Cruise {
	id: number;
	owner_id: number;
	name: string;
	year?: number;
	embark_date?: string;
	disembark_date?: string;
	countries?: string;
	start_port?: string;
	end_port?: string;
	hours_total?: number;
	hours_sail?: number;
	hours_engine?: number;
	hours_over_6bf?: number;
	miles?: number;
	days?: number;
	captain_name?: string;
	yacht_id?: number;
	tidal_waters?: boolean;
	cost_total?: number;
	cost_per_person?: number;
	image_logo_url?: string;
	image_photo_url?: string;
	image_route_url?: string;
	description?: string;
	created_at: string;
	updated_at: string;
}

export interface Yacht {
	id: number;
	owner_id: number;
	name: string;
	registration_no?: string;
	yacht_type?: string;
}

export interface CrewMember {
	id: number;
	owner_id: number;
	user_id?: number;
	full_name: string;
	email?: string;
	patent_number?: string;
}

export interface CrewAssignment {
	id: number;
	cruise_id: number;
	crew_member_id: number;
	role: string;
	patent_number?: string;
	full_name: string;
	email?: string;
}

export interface Training {
	id: number;
	user_id: number;
	date?: string;
	name: string;
	organizer?: string;
	cost?: number;
	url?: string;
}

export interface DashboardStats {
	cruise_count: number;
	total_hours: number;
	total_miles: number;
	total_days: number;
	total_hours_sail: number;
	total_hours_engine: number;
	by_year: YearStats[];
}

export interface YearStats {
	year: number;
	cruise_count: number;
	total_hours: number;
	total_miles: number;
	total_days: number;
}

export interface UploadResponse {
	url: string;
}

export interface VoyageOpinion {
	id: number;
	cruise_id: number;
	crew_member_id: number;
	file_path: string;
	file_format: string;
	full_name: string;
	created_at: string;
}
