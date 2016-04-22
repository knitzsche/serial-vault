/*
 * Copyright (C) 2016-2017 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */
'use strict'

var React = require('react');
var injectIntl = require('react-intl').injectIntl;

var ModelRow = React.createClass({
	render: function() {
		var M = this.props.intl.formatMessage;
		return (
			<tr>
			  <td>
					<a href={'/models/'.concat(this.props.model.id, '/edit')} className="button--secondary" title={M({id: 'edit-model'})}><i className="fa fa-pencil"></i></a>
				</td>
				<td>{this.props.model['brand-id']}</td>
				<td>{this.props.model.model}</td>
				<td>{this.props.model.revision}</td>
				<td>{this.props.model['authority-id']}/{this.props.model['key-id']}</td>
			</tr>
		)
	}
});

module.exports = injectIntl(ModelRow);
