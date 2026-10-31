<?php
// First-install settings for the Order Desk test instance (run once by the image's docker-run.sh).
require_once '/var/www/html/master.inc.php';
require_once DOL_DOCUMENT_ROOT.'/core/lib/admin.lib.php';

$consts = array(
	'MAIN_MONNAIE' => 'DZD',
	// the panel turns a password into the user's DOLAPIKEY once, through /login
	'API_ENABLE_LOGIN_API' => '1',
	// stock moves when a reception is validated; nothing on order validation, shipping or billing
	'STOCK_CALCULATE_ON_RECEPTION' => '1',
	'STOCK_CALCULATE_ON_RECEPTION_CLOSE' => '0',
	'STOCK_CALCULATE_ON_SUPPLIER_DISPATCH_ORDER' => '0',
	'STOCK_CALCULATE_ON_SUPPLIER_VALIDATE_ORDER' => '0',
	'STOCK_CALCULATE_ON_SUPPLIER_BILL' => '0',
	'STOCK_CALCULATE_ON_VALIDATE_ORDER' => '0',
	'STOCK_CALCULATE_ON_BILL' => '0',
	'STOCK_CALCULATE_ON_SHIPMENT' => '0',
	'STOCK_CALCULATE_ON_SHIPMENT_CLOSE' => '0',
	'STOCK_ALLOW_NEGATIVE_TRANSFER' => '1',
	// supplier orders straight to approved (one user, no second approver)
	'SUPPLIER_ORDER_NO_DIRECT_APPROVE' => '0',
);
foreach ($consts as $name => $value) {
	dolibarr_set_const($db, $name, $value, 'chaine', 0, '', $conf->entity);
	print "const $name = $value\n";
}
